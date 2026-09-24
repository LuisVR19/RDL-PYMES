package invoice

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"rdl/billing-api/internal/domain/money"
)

var (
	// ErrWithoutLines: no se emite un documento sin líneas (422 invoice-without-lines).
	ErrWithoutLines = errors.New("invoice: el documento no tiene líneas")
	// ErrInconsistentTotals: los totales guardados no son la suma de las líneas. No debería pasar (los calcula
	// ReplaceLines); si pasa, se niega la emisión en lugar de mandar a Hacienda montos que no cuadran.
	ErrInconsistentTotals = errors.New("invoice: los totales no cuadran con las líneas")
)

// IssueInput son los datos que fija la emisión. El caso de uso los arma con información de la base (cliente,
// secuencia, zona horaria de la organización), nunca del cliente HTTP.
type IssueInput struct {
	Number   string
	IssuedAt time.Time
	// IssueDate es la fecha de negocio de IssuedAt en la zona de la organización (YYYY-MM-DD).
	IssueDate string
	Customer  CustomerSnapshot
	IssuedBy  uuid.UUID
}

// Issue es la transición draft → issued (state-machines/invoice.yaml). Copia el snapshot del cliente, fija número,
// fecha y vencimiento, y congela las líneas tal como están en el borrador (informe 0001 §4.3). Después de esto,
// la base impide cualquier cambio (invoices_guard, guard_draft_children).
func (inv Invoice) Issue(in IssueInput) (Invoice, error) {
	if inv.Status != StatusDraft {
		return Invoice{}, ErrNotDraft
	}
	if len(inv.Lines) == 0 {
		return Invoice{}, ErrWithoutLines
	}
	if err := inv.checkTotals(); err != nil {
		return Invoice{}, err
	}
	number := strings.TrimSpace(in.Number)
	if number == "" || utf8.RuneCountInString(number) > 50 {
		return Invoice{}, fmt.Errorf("invoice: número visible inválido %q", in.Number)
	}
	c := in.Customer
	if c.IdentificationTypeCode == "" || c.IdentificationNumber == "" || c.LegalName == "" {
		// invoices_issued_ck exige el snapshot mínimo del cliente.
		return Invoice{}, errors.New("invoice: snapshot del cliente incompleto")
	}
	due, err := DueDate(in.IssueDate, inv.CreditTermDays)
	if err != nil {
		return Invoice{}, err
	}

	next := inv
	issuedAt := in.IssuedAt.UTC()
	issuedBy := in.IssuedBy
	next.Status = StatusIssued
	next.Number = number
	next.IssuedAt = &issuedAt
	next.IssuedByUserID = &issuedBy
	next.Customer = c
	next.DueDate = due
	return next, nil
}

// DueDate: fecha de emisión + plazo de crédito; de contado (sin plazo), la misma fecha de emisión (informe 0001
// §4.4). Nunca anterior a la emisión (InvoiceIssued v1). TODO(fiscal): qué condiciones de venta exigen plazo
// (fiscal.sale_conditions.requires_credit_term, catálogo vacío).
func DueDate(issueDate string, creditTermDays *int) (string, error) {
	d, err := time.Parse(time.DateOnly, issueDate)
	if err != nil {
		return "", fmt.Errorf("invoice: fecha de emisión %q: %w", issueDate, err)
	}
	if creditTermDays != nil {
		d = d.AddDate(0, 0, *creditTermDays)
	}
	return d.Format(time.DateOnly), nil
}

// checkTotals verifica que cada línea cuadre por dentro (impuesto neto = Σ impuestos − Σ exoneraciones,
// total = subtotal + impuesto neto) y que los totales del documento sean exactamente la suma de las líneas.
func (inv Invoice) checkTotals() error {
	amounts := make([]LineAmounts, 0, len(inv.Lines))
	for _, l := range inv.Lines {
		a := LineAmounts{Discount: l.Discount, Subtotal: l.Subtotal, Tax: l.Tax, Total: l.Total}
		net := decimal.Zero
		for _, t := range l.Taxes {
			ta := TaxAmounts{TaxableBase: t.TaxableBase, Amount: t.Amount, Exoneration: zero()}
			if t.Exoneration != nil {
				ta.Exoneration = t.Exoneration.Amount
			}
			net = net.Add(money.Dec(ta.Amount)).Sub(money.Dec(ta.Exoneration))
			a.Taxes = append(a.Taxes, ta)
		}
		if !net.Equal(money.Dec(l.Tax)) || !money.Dec(l.Subtotal).Add(money.Dec(l.Tax)).Equal(money.Dec(l.Total)) {
			return fmt.Errorf("%w: línea %d", ErrInconsistentTotals, l.Number)
		}
		amounts = append(amounts, a)
	}
	sum, err := CalculateTotals(amounts)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInconsistentTotals, err)
	}
	equal := func(x, y interface{ String() string }) bool { return x.String() == y.String() }
	t := inv.Totals
	if !equal(sum.Subtotal, t.Subtotal) || !equal(sum.Discount, t.Discount) || !equal(sum.Tax, t.Tax) ||
		!equal(sum.Exoneration, t.Exoneration) || !equal(sum.Total, t.Total) {
		return fmt.Errorf("%w: líneas %+v, documento %+v", ErrInconsistentTotals, sum, t)
	}
	return nil
}
