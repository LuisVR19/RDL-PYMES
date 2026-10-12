package app

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"rdl/einvoice-api/internal/domain/permission"
	"rdl/einvoice-api/pkg/tenancy"
)

// Catálogos para formularios y búsqueda de CABYS (planning P5, F4; incremento 5). Todos los roles los leen.

// CatalogEntry es un código de catálogo. Rate y RequiresCreditTerm solo existen en tarifas y condiciones de venta.
type CatalogEntry struct {
	Code               string
	Name               string
	Rate               string // puntos porcentuales, solo en tax-rates
	RequiresCreditTerm *bool  // solo en sale-conditions
}

// CatalogFilter acota cantones y distritos por su padre (Nota 14).
type CatalogFilter struct {
	ProvinceCode string
	CantonCode   string
}

// CabysEntry es un resultado de la búsqueda en la versión vigente del CABYS.
type CabysEntry struct {
	Code        string
	Description string
	TaxRate     string // puntos porcentuales; vacío si el BCCR trae un texto
	TaxLabel    string // «Exento», «na»
}

// CatalogReader lee los catálogos (lectura pública para todos los roles por sus políticas *_read).
type CatalogReader interface {
	List(ctx context.Context, catalog string, f CatalogFilter) ([]CatalogEntry, error)
	SearchCabys(ctx context.Context, q string, limit int) ([]CabysEntry, error)
}

// Catalogs son los nombres públicos de catálogo (los de openapi/fiscal.yaml más los que este repo agrega) y su
// tabla. Lista cerrada: el nombre llega por la URL y la tabla va a un SQL.
var Catalogs = map[string]string{
	"identification-types":       "identification_types",
	"tax-types":                  "tax_types",
	"tax-rates":                  "tax_rates",
	"units-of-measure":           "units_of_measure",
	"sale-conditions":            "sale_conditions",
	"payment-methods":            "payment_methods",
	"exoneration-document-types": "exoneration_document_types",
	"document-types":             "document_types",
	// Agregados por este repo (TODO(contratos): sumarlos al enum de getFiscalCatalog).
	"currencies":               "currencies",
	"reference-codes":          "reference_codes",
	"reference-document-types": "reference_document_types",
	"discount-codes":           "discount_codes",
	"exoneration-institutions": "exoneration_institutions",
	"provinces":                "provinces",
	"cantons":                  "cantons",
	"districts":                "districts",
}

var (
	provinceRe = regexp.MustCompile(`^[0-9]$`)
	cantonRe   = regexp.MustCompile(`^[0-9]{2}$`)
)

type ListCatalog struct{ txm TxManager }

func NewListCatalog(txm TxManager) *ListCatalog { return &ListCatalog{txm: txm} }

// Execute devuelve los códigos activos del catálogo. Uno desconocido es ErrNotFound. cantons exige provinceCode y
// districts exige provinceCode y cantonCode: sin padre serían cientos de filas sin contexto para un formulario.
func (uc *ListCatalog) Execute(ctx context.Context, t tenancy.Context, catalog string, f CatalogFilter) ([]CatalogEntry, error) {
	if err := authorize(t, permission.CatalogsRead); err != nil {
		return nil, err
	}
	if _, ok := Catalogs[catalog]; !ok {
		return nil, ErrNotFound
	}
	var errs ValidationErrors
	if catalog == "cantons" || catalog == "districts" {
		if !provinceRe.MatchString(f.ProvinceCode) {
			errs = append(errs, ValidationError{Field: "provinceCode", Message: "es obligatorio: un dígito (Nota 14)"})
		}
	}
	if catalog == "districts" && !cantonRe.MatchString(f.CantonCode) {
		errs = append(errs, ValidationError{Field: "cantonCode", Message: "es obligatorio: dos dígitos (Nota 14)"})
	}
	if len(errs) > 0 {
		return nil, errs
	}
	var out []CatalogEntry
	err := uc.txm.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		var err error
		out, err = tx.Catalogs().List(ctx, catalog, f)
		return err
	})
	return out, err
}

// Límites de la búsqueda (openapi/fiscal.yaml: q de 2 a 100; Limit de convenciones §9).
const (
	CabysMinQuery     = 2
	CabysMaxQuery     = 100
	CabysDefaultLimit = 20
	CabysMaxLimit     = 100
)

type SearchCabys struct{ txm TxManager }

func NewSearchCabys(txm TxManager) *SearchCabys { return &SearchCabys{txm: txm} }

// Execute busca en la versión vigente: si q son solo dígitos, por prefijo del código (los niveles del CABYS son
// prefijos); si no, por texto en la descripción (índice trigram de la baseline).
func (uc *SearchCabys) Execute(ctx context.Context, t tenancy.Context, q string, limit int) ([]CabysEntry, error) {
	if err := authorize(t, permission.CatalogsRead); err != nil {
		return nil, err
	}
	q = strings.TrimSpace(q)
	var errs ValidationErrors
	if n := utf8.RuneCountInString(q); n < CabysMinQuery || n > CabysMaxQuery {
		errs = append(errs, ValidationError{Field: "q", Message: fmt.Sprintf("debe tener de %d a %d caracteres", CabysMinQuery, CabysMaxQuery)})
	}
	if limit == 0 {
		limit = CabysDefaultLimit
	}
	if limit < 1 || limit > CabysMaxLimit {
		errs = append(errs, ValidationError{Field: "limit", Message: fmt.Sprintf("debe estar entre 1 y %d", CabysMaxLimit)})
	}
	if len(errs) > 0 {
		return nil, errs
	}
	var out []CabysEntry
	err := uc.txm.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		var err error
		out, err = tx.Catalogs().SearchCabys(ctx, q, limit)
		return err
	})
	return out, err
}
