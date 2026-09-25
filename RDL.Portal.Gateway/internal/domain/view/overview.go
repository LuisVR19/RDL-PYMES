// Package view son los modelos de vista del portal y las reglas de degradación. Es dominio de PRESENTACIÓN:
// junta lo que cada API respondió y decide qué mostrar cuando una no contestó. No calcula montos ni impuestos,
// no decide estados y no conoce HTTP: los montos viajan como string decimal, tal como los dio su API dueña.
package view

// Availability dice si una parte de la vista se pudo traer. Es un concepto del Portal Gateway y vive aparte del
// `status` de cada API: mezclarlos metería un estado inventado en la máquina de estados del documento electrónico
// o de la cuenta por cobrar, y el contrato manda (ADR 0004).
type Availability string

const (
	// Available: la API dueña respondió y el dato está.
	Available Availability = "available"
	// Absent: la API dueña respondió que no existe. No es una falla: una factura en borrador todavía no tiene
	// documento electrónico ni cuenta por cobrar.
	Absent Availability = "absent"
	// Unavailable: no se pudo saber (API caída, timeout o todavía sin desplegar). El portal muestra la factura
	// igual y avisa que esa parte no se pudo consultar (criterio 2 de la arquitectura).
	Unavailable Availability = "unavailable"
)

// InvoiceSummary es lo que Billing sabe del documento: la parte PRINCIPAL de la vista.
// Si esta falta, no hay vista que mostrar.
type InvoiceSummary struct {
	ID                 string
	DocumentType       string
	Number             string // vacío mientras es borrador
	Status             string
	RequiresCorrection bool
	CustomerLegalName  string // vacío antes de emitir: el snapshot se toma al emitir
	Currency           string
	Total              string // decimal exacto como string, nunca float
}

// FiscalStatus es lo que E-Invoice sabe del documento electrónico de esa factura.
type FiscalStatus struct {
	ElectronicDocumentID  string
	Status                string
	HaciendaStatusMessage string
}

// Balance es lo que Receivables sabe de la cuenta por cobrar de esa factura.
type Balance struct {
	ReceivableID  string
	Status        string
	Currency      string
	BalanceAmount string // decimal exacto como string
	DueOn         string // fecha de negocio (YYYY-MM-DD)
}

// FiscalPart y BalancePart son partes SECUNDARIAS: siempre traen su disponibilidad y, solo si es Available,
// el dato. El invariante lo fija el constructor para que ningún handler pueda publicar un dato a medias.
type FiscalPart struct {
	Availability Availability
	Status       *FiscalStatus
}

type BalancePart struct {
	Availability Availability
	Balance      *Balance
}

func NewFiscalPart(a Availability, s *FiscalStatus) FiscalPart {
	if a != Available || s == nil {
		return FiscalPart{Availability: withoutValue(a, s == nil)}
	}
	return FiscalPart{Availability: Available, Status: s}
}

func NewBalancePart(a Availability, b *Balance) BalancePart {
	if a != Available || b == nil {
		return BalancePart{Availability: withoutValue(a, b == nil)}
	}
	return BalancePart{Availability: Available, Balance: b}
}

// withoutValue evita el estado imposible "disponible pero sin dato": si la API dijo que sí pero no mandó nada,
// para el portal es como no haber podido consultarla.
func withoutValue(a Availability, missing bool) Availability {
	if a == Available && missing {
		return Unavailable
	}
	return a
}

// InvoiceOverview es la vista transversal de la arquitectura 2.2:
// total de Billing + estado de Hacienda + saldo, sin crear dependencias entre dominios.
type InvoiceOverview struct {
	Invoice    InvoiceSummary
	Fiscal     FiscalPart
	Receivable BalancePart
}

// Degraded indica que alguna parte secundaria no se pudo consultar. Lo usan el log y las métricas;
// el portal se guía por la Availability de cada parte.
func (o InvoiceOverview) Degraded() bool {
	return o.Fiscal.Availability == Unavailable || o.Receivable.Availability == Unavailable
}
