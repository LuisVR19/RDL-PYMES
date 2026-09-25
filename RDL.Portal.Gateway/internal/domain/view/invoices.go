package view

// InvoiceRow es una fila del listado de documentos (pantalla 12), tal como la dio Billing. Es la parte PRINCIPAL:
// sin la página de Billing no hay listado. Los instantes y las fechas viajan como vinieron (RFC 3339 con Z y
// YYYY-MM-DD); los montos, como string decimal.
type InvoiceRow struct {
	ID                 string
	DocumentType       string
	Number             string // vacío mientras es borrador
	Status             string
	RequiresCorrection bool
	CustomerID         string
	CustomerLegalName  string // vacío antes de emitir: el snapshot se toma al emitir
	IssuedAt           string // vacío mientras es borrador
	DueDate            string // vacío si no aplica
	Currency           string
	Total              string
	CreatedAt          string
}

// InvoiceRows es una página tal como la devolvió Billing: sus filas en su orden y su cursor opaco.
type InvoiceRows struct {
	Rows       []InvoiceRow
	NextCursor string // vacío = no hay más
}

// InvoiceListItem es una fila del listado con sus partes secundarias, con la misma regla que la vista
// transversal: cada parte trae su disponibilidad y el dato solo si está.
type InvoiceListItem struct {
	Invoice    InvoiceRow
	Fiscal     FiscalPart
	Receivable BalancePart
}

type InvoicePage struct {
	Items      []InvoiceListItem
	NextCursor string
}

// Batch es el resultado de una consulta por lote a una API secundaria: los datos que trajo, por id de factura,
// y si la consulta se pudo hacer. Un lote se consulta entero o no se consulta: no hay "mitad disponible".
type Batch[T any] struct {
	Found     map[string]T
	Available bool
}

// JoinInvoicePage arma el listado. Por fila:
//   - si el lote se pudo consultar y trae la factura → Available con su dato;
//   - si se pudo consultar y no la trae → Absent (un borrador no tiene documento electrónico ni saldo);
//   - si el lote no se pudo consultar → Unavailable en todas las filas, sin romper la tabla.
//
// No reordena, no filtra y no suma: el orden y el cursor son los de Billing.
func JoinInvoicePage(rows InvoiceRows, fiscal Batch[FiscalStatus], balances Batch[Balance]) InvoicePage {
	page := InvoicePage{Items: make([]InvoiceListItem, 0, len(rows.Rows)), NextCursor: rows.NextCursor}
	for _, r := range rows.Rows {
		item := InvoiceListItem{Invoice: r}
		f, ok := fiscal.Found[r.ID]
		item.Fiscal = NewFiscalPart(batchAvailability(fiscal.Available, ok), &f)
		b, ok := balances.Found[r.ID]
		item.Receivable = NewBalancePart(batchAvailability(balances.Available, ok), &b)
		page.Items = append(page.Items, item)
	}
	return page
}

func batchAvailability(consulted, found bool) Availability {
	switch {
	case !consulted:
		return Unavailable
	case found:
		return Available
	default:
		return Absent
	}
}

// Degraded indica que alguna parte de alguna fila no se pudo consultar (para el log y las métricas).
func (p InvoicePage) Degraded() bool {
	for _, it := range p.Items {
		if it.Fiscal.Availability == Unavailable || it.Receivable.Availability == Unavailable {
			return true
		}
	}
	return false
}

// InvoiceIDs devuelve los ids de la página, en orden, para pedir los lotes.
func (r InvoiceRows) InvoiceIDs() []string {
	ids := make([]string, 0, len(r.Rows))
	for _, row := range r.Rows {
		ids = append(ids, row.ID)
	}
	return ids
}
