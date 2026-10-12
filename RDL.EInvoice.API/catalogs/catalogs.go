// Package catalogs son los catálogos de Hacienda transcritos de docs/hacienda/ (CSV con la nota, la página y el
// sha256 del PDF de origen en la cabecera). Los carga cmd/import-catalogs; nunca se escriben a mano en la base.
// Un catálogo nuevo es un CSV más aquí, con su cita; la tabla destino tiene que estar en Tables.
package catalogs

import (
	"bufio"
	"embed"
	"encoding/csv"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"sort"
	"strings"
)

//go:embed *.csv
var files embed.FS

// Tables son las tablas de catálogo que el importador puede escribir, con las columnas adicionales que admite cada
// una además de code y name. Lista cerrada: el nombre de la tabla y de las columnas salen del CSV y van a un SQL.
var Tables = map[string][]string{
	"document_types":             nil,
	"identification_types":       nil,
	"payment_methods":            nil,
	"sale_conditions":            {"requires_credit_term"},
	"tax_types":                  nil,
	"tax_rates":                  {"rate"},
	"reference_codes":            nil,
	"reference_document_types":   nil,
	"exoneration_document_types": nil,
	"units_of_measure":           nil,
	"discount_codes":             nil,
	"exoneration_institutions":   nil,
	"currencies":                 nil,
}

// valueRe valida cada columna adicional: booleanos o decimales con punto (sin float en ningún lado).
var valueRe = map[string]*regexp.Regexp{
	"requires_credit_term": regexp.MustCompile(`^(true|false)$`),
	"rate":                 regexp.MustCompile(`^[0-9]{1,3}(\.[0-9]{1,4})?$`),
}

// Catalog es un CSV ya leído.
type Catalog struct {
	File         string
	Table        string
	Source       string
	SourceSHA256 string
	Version      string
	Citation     string
	Columns      []string // columnas adicionales, en orden
	Rows         []Row
}

type Row struct {
	Code, Name string
	Extra      []string // valores de Columns, en el mismo orden
}

var sha256Re = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Load lee y valida todos los catálogos embebidos, en orden de archivo.
func Load() ([]Catalog, error) {
	names, err := fs.Glob(files, "*.csv")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	out := make([]Catalog, 0, len(names))
	for _, name := range names {
		raw, err := files.ReadFile(name)
		if err != nil {
			return nil, err
		}
		c, err := Parse(name, string(raw))
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// Parse lee la cabecera de comentarios (# clave: valor) y las filas code,name.
func Parse(name, raw string) (Catalog, error) {
	c := Catalog{File: name}
	var body strings.Builder
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := sc.Text()
		if rest, ok := strings.CutPrefix(line, "#"); ok {
			k, v, _ := strings.Cut(strings.TrimSpace(rest), ":")
			v = strings.TrimSpace(v)
			switch strings.TrimSpace(k) {
			case "tabla":
				c.Table = v
			case "fuente":
				c.Source = v
			case "fuente_sha256":
				c.SourceSHA256 = v
			case "version":
				c.Version = v
			case "cita":
				c.Citation = v
			}
			continue
		}
		body.WriteString(line + "\n")
	}
	if err := sc.Err(); err != nil {
		return Catalog{}, err
	}
	var errs []error
	allowed, ok := Tables[c.Table]
	if !ok {
		errs = append(errs, fmt.Errorf("tabla %q no admitida", c.Table))
	}
	if c.Source == "" || c.Version == "" || c.Citation == "" {
		errs = append(errs, errors.New("faltan fuente, version o cita en la cabecera"))
	}
	if !sha256Re.MatchString(c.SourceSHA256) {
		errs = append(errs, errors.New("fuente_sha256 inválido"))
	}
	records, err := csv.NewReader(strings.NewReader(body.String())).ReadAll()
	if err != nil {
		errs = append(errs, err)
	}
	seen := map[string]bool{}
	for i, r := range records {
		if i == 0 {
			if len(r) < 2 || r[0] != "code" || r[1] != "name" {
				errs = append(errs, errors.New("el encabezado debe empezar con code,name"))
				break
			}
			for _, col := range r[2:] {
				if !slices.Contains(allowed, col) {
					errs = append(errs, fmt.Errorf("columna %q no admitida en %s", col, c.Table))
				}
			}
			c.Columns = r[2:]
			continue
		}
		if len(r) != 2+len(c.Columns) || strings.TrimSpace(r[0]) == "" || strings.TrimSpace(r[1]) == "" {
			errs = append(errs, fmt.Errorf("fila %d incompleta", i))
			continue
		}
		for j, col := range c.Columns {
			if re := valueRe[col]; re != nil && !re.MatchString(r[2+j]) {
				errs = append(errs, fmt.Errorf("fila %d: %s %q inválido", i, col, r[2+j]))
			}
		}
		// Sin normalizar mayúsculas: hay catálogos que las distinguen (Nota 15, informe 0001 §2).
		if seen[r[0]] {
			errs = append(errs, fmt.Errorf("código %q repetido", r[0]))
		}
		seen[r[0]] = true
		c.Rows = append(c.Rows, Row{Code: r[0], Name: r[1], Extra: r[2:]})
	}
	if len(c.Rows) == 0 {
		errs = append(errs, errors.New("sin filas"))
	}
	if len(errs) > 0 {
		return Catalog{}, fmt.Errorf("%s: %w", name, errors.Join(errs...))
	}
	return c, nil
}
