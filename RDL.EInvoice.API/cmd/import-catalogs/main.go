// Command import-catalogs carga los catálogos de Hacienda y el CABYS en las tablas fiscal.*, y registra cada carga en
// fiscal.catalog_imports con su versión y el sha256 de la fuente.
//
//   - catalogs/*.csv: Notas del Anexo 1 transcritas (cada una cita nota, página y sha256 del PDF).
//   - docs/hacienda/catalogos/Codificacionubicacion_V4.4.xlsx: provincias, cantones y distritos (Nota 14).
//   - docs/hacienda/catalogos/CABYS_2025-…xlsx: CABYS del BCCR (Nota 17), como versión nueva y vigente.
//
// Uso: import-catalogs [-dry-run] [-hacienda docs/hacienda] [-skip-cabys]
// Conexión: igual que cmd/migrate (MIGRATE_DATABASE_URL o DB_POOLER_HOST con el login fiscal_migrate, modo sesión).
// Corre como fiscal_migrator, dueño de los catálogos (informe 0002 §6). Idempotente: actualiza nombres, no borra
// códigos y no repite una versión ya registrada.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"rdl/einvoice-api/catalogs"
	"rdl/einvoice-api/internal/platform/config"
)

const (
	expectedRole = "fiscal_migrator"
	loginRole    = "fiscal_migrate"
)

// extraTypes es el tipo SQL de cada columna adicional admitida (catalogs.Tables).
var extraTypes = map[string]string{"requires_credit_term": "boolean", "rate": "numeric"}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "import-catalogs:", err)
		os.Exit(1)
	}
}

func run() error {
	dryRun := flag.Bool("dry-run", false, "valida las fuentes y muestra lo que cargaría, sin conectarse")
	haciendaDir := flag.String("hacienda", "docs/hacienda", "directorio de las fuentes de Hacienda")
	skipCabys := flag.Bool("skip-cabys", false, "no carga el CABYS (tarda unos segundos)")
	flag.Parse()

	cs, err := catalogs.Load()
	if err != nil {
		return err
	}
	locations, err := readLocations(*haciendaDir)
	if err != nil {
		return err
	}
	var cabys []catalogs.CabysItem
	if !*skipCabys {
		if cabys, err = readCabys(*haciendaDir); err != nil {
			return err
		}
	}
	if *dryRun {
		for _, c := range cs {
			fmt.Printf("%-28s %5d filas  %s · %s\n", c.Table, len(c.Rows), c.File, c.Citation)
		}
		fmt.Printf("%-28s %5d distritos  %s · %s\n", "provinces/cantons/districts", len(locations), catalogs.LocationSource.File, catalogs.LocationSource.Cite)
		fmt.Printf("%-28s %5d códigos  %s · %s\n", "cabys_items", len(cabys), catalogs.CabysSource.File, catalogs.CabysSource.Cite)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	conn, err := connect(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	return pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
		for _, c := range cs {
			if err := loadCSV(ctx, tx, c); err != nil {
				return fmt.Errorf("%s: %w", c.File, err)
			}
		}
		if err := loadLocations(ctx, tx, locations); err != nil {
			return fmt.Errorf("ubicación: %w", err)
		}
		if !*skipCabys {
			if err := loadCabys(ctx, tx, cabys); err != nil {
				return fmt.Errorf("CABYS: %w", err)
			}
		}
		return nil
	})
}

func connect(ctx context.Context) (*pgx.Conn, error) {
	if err := config.LoadDotEnv(".env"); err != nil {
		return nil, err
	}
	url := os.Getenv("MIGRATE_DATABASE_URL")
	if url == "" {
		var err error
		url, err = config.PoolerURL(os.Getenv("SUPABASE_URL"), os.Getenv("DB_POOLER_HOST"), loginRole, config.PoolerSessionPort)
		if err != nil {
			return nil, err
		}
	}
	connCfg, err := pgx.ParseConfig(url)
	if err != nil {
		return nil, errors.New("MIGRATE_DATABASE_URL inválida")
	}
	if pw := os.Getenv("MIGRATE_DB_PASSWORD"); pw != "" {
		connCfg.Password = pw
	}
	connCfg.DefaultQueryExecMode = pgx.QueryExecModeExec
	conn, err := pgx.ConnectConfig(ctx, connCfg)
	if err != nil {
		return nil, fmt.Errorf("conectando: %w", err)
	}
	var role string
	if err := conn.QueryRow(ctx, "select current_user").Scan(&role); err != nil {
		_ = conn.Close(ctx)
		return nil, err
	}
	if role != expectedRole {
		_ = conn.Close(ctx)
		return nil, fmt.Errorf("la sesión corre como %q; se esperaba %q (revise ALTER ROLE ... SET role)", role, expectedRole)
	}
	return conn, nil
}

// loadCSV hace un solo upsert por catálogo con arreglos de texto (ADR 0004 de Platform: arreglos como texto).
func loadCSV(ctx context.Context, tx pgx.Tx, c catalogs.Catalog) error {
	allowed, ok := catalogs.Tables[c.Table]
	if !ok {
		return fmt.Errorf("tabla %q no admitida", c.Table)
	}
	cols := []string{"code", "name"}
	selects := []string{"u.c1", "u.c2"}
	updates := []string{"name = excluded.name", "is_active = true"}
	args := []any{codesOf(c, -1), namesOf(c)}
	for j, col := range c.Columns {
		typ, known := extraTypes[col]
		if !known || !contains(allowed, col) {
			return fmt.Errorf("columna %q no admitida", col)
		}
		cols = append(cols, col)
		selects = append(selects, fmt.Sprintf("u.c%d::%s", j+3, typ))
		updates = append(updates, col+" = excluded."+col)
		args = append(args, codesOf(c, j))
	}
	unnestArgs := make([]string, len(args))
	aliases := make([]string, len(args))
	for i := range args {
		unnestArgs[i] = fmt.Sprintf("$%d::text[]", i+1)
		aliases[i] = fmt.Sprintf("c%d", i+1)
	}
	table := pgx.Identifier{"fiscal", c.Table}.Sanitize()
	// #nosec G202 -- tabla y columnas salen de listas cerradas (catalogs.Tables, extraTypes); los valores van como parámetros
	q := "insert into " + table + " (" + strings.Join(cols, ", ") + ", is_active) " +
		"select " + strings.Join(selects, ", ") + ", true from unnest(" + strings.Join(unnestArgs, ", ") + ") as u(" +
		strings.Join(aliases, ", ") + ") on conflict (code) do update set " + strings.Join(updates, ", ")
	if _, err := tx.Exec(ctx, q, args...); err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(fmt.Sprint(c.Rows)))
	version := c.Version + " · csv " + hex.EncodeToString(sum[:])[:12]
	return record(ctx, tx, c.Table, version, c.Source+" ("+strings.TrimSuffix(c.Citation, ".")+")", c.SourceSHA256, len(c.Rows))
}

func codesOf(c catalogs.Catalog, extra int) []string {
	out := make([]string, len(c.Rows))
	for i, r := range c.Rows {
		if extra < 0 {
			out[i] = r.Code
		} else {
			out[i] = r.Extra[extra]
		}
	}
	return out
}

func namesOf(c catalogs.Catalog) []string {
	out := make([]string, len(c.Rows))
	for i, r := range c.Rows {
		out[i] = r.Name
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func readLocations(dir string) ([]catalogs.Location, error) {
	f, err := catalogs.LocationSource.Open(dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return catalogs.LoadLocations(f)
}

func readCabys(dir string) ([]catalogs.CabysItem, error) {
	f, err := catalogs.CabysSource.Open(dir)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return catalogs.LoadCabys(f)
}

// loadLocations: provincias, cantones y distritos, con la FK compuesta de la baseline (cantón → provincia, distrito →
// cantón). Tres upserts, uno por nivel.
func loadLocations(ctx context.Context, tx pgx.Tx, ls []catalogs.Location) error {
	var pc, pn, cp, cc, cn, dp, dc, dd, dn []string
	seenP, seenC := map[string]bool{}, map[string]bool{}
	for _, l := range ls {
		if !seenP[l.ProvinceCode] {
			seenP[l.ProvinceCode] = true
			pc, pn = append(pc, l.ProvinceCode), append(pn, l.ProvinceName)
		}
		if k := l.ProvinceCode + l.CantonCode; !seenC[k] {
			seenC[k] = true
			cp, cc, cn = append(cp, l.ProvinceCode), append(cc, l.CantonCode), append(cn, l.CantonName)
		}
		dp, dc, dd, dn = append(dp, l.ProvinceCode), append(dc, l.CantonCode), append(dd, l.DistrictCode), append(dn, l.DistrictName)
	}
	steps := []struct {
		q    string
		args []any
	}{
		{`insert into fiscal.provinces (code, name) select * from unnest($1::text[], $2::text[])
		  on conflict (code) do update set name = excluded.name`, []any{pc, pn}},
		{`insert into fiscal.cantons (province_code, code, name) select * from unnest($1::text[], $2::text[], $3::text[])
		  on conflict (province_code, code) do update set name = excluded.name`, []any{cp, cc, cn}},
		{`insert into fiscal.districts (province_code, canton_code, code, name)
		  select * from unnest($1::text[], $2::text[], $3::text[], $4::text[])
		  on conflict (province_code, canton_code, code) do update set name = excluded.name`, []any{dp, dc, dd, dn}},
	}
	for _, s := range steps {
		if _, err := tx.Exec(ctx, s.q, s.args...); err != nil {
			return err
		}
	}
	src := catalogs.LocationSource
	fmt.Printf("%-28s %d provincias, %d cantones, %d distritos\n", "provinces/cantons/districts", len(pc), len(cc), len(dd))
	return record(ctx, tx, "locations", src.Version, src.File+" ("+src.Cite+")", src.SHA256, len(dd))
}

// loadCabys registra la versión del BCCR como nueva y vigente, con COPY para los ~20 000 códigos. Si la versión ya
// está, no hace nada: una versión publicada del CABYS no cambia.
func loadCabys(ctx context.Context, tx pgx.Tx, items []catalogs.CabysItem) error {
	src := catalogs.CabysSource
	var exists bool
	if err := tx.QueryRow(ctx, "select exists (select 1 from fiscal.cabys_versions where version_label = $1)", src.Version).Scan(&exists); err != nil {
		return err
	}
	if exists {
		fmt.Printf("%-28s versión %s (ya registrada)\n", "cabys_items", src.Version)
		return nil
	}
	versionID := uuid.New()
	if _, err := tx.Exec(ctx, `insert into fiscal.cabys_versions (id, version_label, published_on, source_sha256, is_current)
		values ($1, $2, '2025-04-01', $3, false)`, versionID, src.Version, src.SHA256); err != nil {
		return err
	}
	rows := make([][]any, len(items))
	for i, it := range items {
		var rate any
		if it.TaxRate != nil {
			rate = numeric(*it.TaxRate)
		}
		var label any
		if it.TaxLabel != "" {
			label = it.TaxLabel
		}
		rows[i] = []any{versionID, it.Code, it.Description, rate, label}
	}
	n, err := tx.CopyFrom(ctx, pgx.Identifier{"fiscal", "cabys_items"},
		[]string{"version_id", "code", "description", "tax_rate", "tax_label"}, pgx.CopyFromRows(rows))
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "update fiscal.cabys_versions set is_current = (id = $1)", versionID); err != nil {
		return err
	}
	fmt.Printf("%-28s %d códigos, versión %s (registrada y vigente)\n", "cabys_items", n, src.Version)
	return record(ctx, tx, "cabys", src.Version, src.File+" ("+src.Cite+")", src.SHA256, int(n))
}

// numeric arma el numeric de Postgres desde el decimal exacto (coeficiente y exponente), sin pasar por float.
func numeric(d decimal.Decimal) pgtype.Numeric {
	return pgtype.Numeric{Int: d.Coefficient(), Exp: d.Exponent(), Valid: true}
}

func record(ctx context.Context, tx pgx.Tx, catalog, version, source, sha string, rows int) error {
	tag, err := tx.Exec(ctx, `insert into fiscal.catalog_imports (catalog, version_label, source_file, source_sha256, row_count)
		values ($1, $2, $3, $4, $5) on conflict (catalog, version_label, source_sha256) do nothing`,
		catalog, version, source, sha, rows)
	if err != nil {
		return fmt.Errorf("registrando la carga: %w", err)
	}
	state := "ya registrada"
	if tag.RowsAffected() == 1 {
		state = "registrada"
	}
	if catalog != "locations" && catalog != "cabys" {
		fmt.Printf("%-28s %5d filas  versión %s (%s)\n", catalog, rows, version, state)
	}
	return nil
}
