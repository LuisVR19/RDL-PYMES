package postgres

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"

	"rdl/einvoice-api/internal/app"
)

// catalogs lee los catálogos con SQL armado sobre la lista cerrada app.Catalogs (el nombre llega por la URL); los
// valores siempre van como parámetros.
type catalogs struct{ tx pgx.Tx }

var digitsOnly = regexp.MustCompile(`^[0-9]+$`)

func (c catalogs) List(ctx context.Context, catalog string, f app.CatalogFilter) ([]app.CatalogEntry, error) {
	table, ok := app.Catalogs[catalog]
	if !ok {
		return nil, app.ErrNotFound
	}
	var q string
	var args []any
	switch table {
	case "provinces":
		q = "select code, name, null::text, null::boolean from fiscal.provinces order by code"
	case "cantons":
		q = "select code, name, null::text, null::boolean from fiscal.cantons where province_code = $1 order by code"
		args = []any{f.ProvinceCode}
	case "districts":
		q = `select code, name, null::text, null::boolean from fiscal.districts
		      where province_code = $1 and canton_code = $2 order by code`
		args = []any{f.ProvinceCode, f.CantonCode}
	case "tax_rates":
		q = "select code, name, trim_scale(rate)::text, null::boolean from fiscal.tax_rates where is_active order by code"
	case "sale_conditions":
		q = "select code, name, null::text, requires_credit_term from fiscal.sale_conditions where is_active order by code"
	default:
		// #nosec G202 -- table sale de app.Catalogs (lista cerrada) y va sanitizada
		q = "select code, name, null::text, null::boolean from " + pgx.Identifier{"fiscal", table}.Sanitize() +
			" where is_active order by code"
	}
	rows, err := c.tx.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("leyendo el catálogo %s: %w", catalog, err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.CatalogEntry, error) {
		var e app.CatalogEntry
		var rate *string
		err := r.Scan(&e.Code, &e.Name, &rate, &e.RequiresCreditTerm)
		if rate != nil {
			e.Rate = *rate
		}
		return e, err
	})
	if err != nil {
		return nil, fmt.Errorf("leyendo el catálogo %s: %w", catalog, err)
	}
	return out, nil
}

// SearchCabys busca en la versión vigente. Por código: prefijo. Por texto: ILIKE con el índice trigram, primero las
// descripciones que empiezan con el texto. El texto se escapa para que % y _ no sean comodines.
func (c catalogs) SearchCabys(ctx context.Context, q string, limit int) ([]app.CabysEntry, error) {
	var sql string
	var arg string
	if digitsOnly.MatchString(q) {
		sql = `select i.code::text, i.description, trim_scale(i.tax_rate)::text, i.tax_label
		         from fiscal.cabys_items i join fiscal.cabys_versions v on v.id = i.version_id and v.is_current
		        where i.code like $1 || '%' order by i.code limit $2`
		arg = q
	} else {
		sql = `select i.code::text, i.description, trim_scale(i.tax_rate)::text, i.tax_label
		         from fiscal.cabys_items i join fiscal.cabys_versions v on v.id = i.version_id and v.is_current
		        where i.description ilike '%' || $1 || '%'
		        order by (i.description ilike $1 || '%') desc, length(i.description), i.code limit $2`
		arg = escapeLike(q)
	}
	rows, err := c.tx.Query(ctx, sql, arg, limit)
	if err != nil {
		return nil, fmt.Errorf("buscando en el CABYS: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (app.CabysEntry, error) {
		var e app.CabysEntry
		var rate, label *string
		err := r.Scan(&e.Code, &e.Description, &rate, &label)
		if rate != nil {
			e.TaxRate = *rate
		}
		if label != nil {
			e.TaxLabel = *label
		}
		return e, err
	})
	if err != nil {
		return nil, fmt.Errorf("buscando en el CABYS: %w", err)
	}
	return out, nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
