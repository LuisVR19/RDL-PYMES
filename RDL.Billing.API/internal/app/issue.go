package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/numbering"
	"rdl/billing-api/internal/domain/permission"
	"rdl/billing-api/pkg/tenancy"
)

// IssueInvoice es draft → issued (POST /v1/invoices/{id}/issue), el corazón de Billing. En UNA transacción:
// revalida cliente, sucursal y productos; asigna el número visible bajo candado; copia el snapshot del cliente; guarda
// la transición, el historial, el audit y InvoiceIssued v1 (validado contra su schema) en el outbox. Responde sin
// llamar a ningún otro servicio: emite aunque E-Invoice esté caída (criterio 2). Idempotente por Idempotency-Key.
type IssueInvoice struct {
	tx  TxManager
	now func() time.Time
}

func NewIssueInvoice(tx TxManager) *IssueInvoice {
	return &IssueInvoice{tx: tx, now: time.Now}
}

type IssueResult struct {
	Invoice  invoice.Invoice
	Replayed bool
}

func (uc *IssueInvoice) Execute(ctx context.Context, t tenancy.Context, idempotencyKey string, id uuid.UUID) (IssueResult, error) {
	if err := authorize(t, permission.InvoicesIssue); err != nil {
		return IssueResult{}, err
	}
	hash, err := requestHash(map[string]string{"issue": id.String()})
	if err != nil {
		return IssueResult{}, err
	}
	var res IssueResult
	err = uc.tx.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		// Primero la clave: un reintento de una emisión que ya se confirmó responde lo mismo, no 409.
		prevID, err := replayed(ctx, tx, t.OrganizationID(), idempotencyKey, hash, "invoiceId")
		if err != nil {
			return err
		}
		if prevID != uuid.Nil {
			inv, err := tx.Invoices().Get(ctx, t.OrganizationID(), prevID)
			res = IssueResult{Invoice: inv, Replayed: true}
			return err
		}

		draft, err := tx.Invoices().GetForUpdate(ctx, t.OrganizationID(), id)
		if err != nil {
			return err
		}
		if draft.Status != invoice.StatusDraft {
			return invoice.ErrNotDraft
		}
		if len(draft.Lines) == 0 {
			return invoice.ErrWithoutLines
		}
		snapshot, err := uc.revalidate(ctx, tx, t.OrganizationID(), draft)
		if err != nil {
			return err
		}
		settings, err := tx.Catalog().Organization(ctx, t.OrganizationID())
		if err != nil {
			return err
		}
		loc, err := time.LoadLocation(settings.Timezone)
		if err != nil {
			return fmt.Errorf("zona horaria de la organización %q: %w", settings.Timezone, err)
		}
		// Precisión de microsegundos, la de timestamptz: el evento y la base guardan el mismo instante.
		issuedAt := uc.now().UTC().Truncate(time.Microsecond)
		issueDate := issuedAt.In(loc).Format(time.DateOnly)

		number, err := assignNumber(ctx, tx, t.OrganizationID(), draft)
		if err != nil {
			return err
		}
		issued, err := draft.Issue(invoice.IssueInput{
			Number: number, IssuedAt: issuedAt, IssueDate: issueDate, Customer: snapshot, IssuedBy: t.UserID(),
		})
		if err != nil {
			return err
		}
		if issued, err = tx.Invoices().MarkIssued(ctx, issued); err != nil {
			return err
		}
		userID := t.UserID()
		if err := tx.Invoices().AddStatusChange(ctx, t.OrganizationID(), id, StatusChange{
			From: invoice.StatusDraft, To: invoice.StatusIssued, ChangedByUserID: &userID, ChangedAt: issuedAt,
		}); err != nil {
			return err
		}
		if err := tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: t.OrganizationID(), ActorUserID: t.UserID(),
			Action: "invoice.issued", EntityType: "invoice", EntityID: id,
			Before: map[string]any{"status": string(invoice.StatusDraft)},
			After: map[string]any{
				"status": string(invoice.StatusIssued), "number": issued.Number, "issuedAt": issuedAt.Format(time.RFC3339Nano),
				"dueDate": issued.DueDate, "total": issued.Totals.Total.String(), "currency": issued.Currency.String(),
			},
		}); err != nil {
			return err
		}
		if err := tx.Outbox().InvoiceIssued(ctx, issued, issueDate); err != nil {
			return err
		}
		if err := tx.Idempotency().Complete(ctx, t.OrganizationID(), idempotencyKey, IdempotencyRecord{
			RequestHash: hash, Status: http.StatusCreated, Result: map[string]string{"invoiceId": id.String()},
		}); err != nil {
			return err
		}
		res = IssueResult{Invoice: issued}
		return nil
	})
	return res, err
}

// revalidate comprueba, al momento de emitir, lo que pudo cambiar desde que se armó el borrador: el cliente sigue
// activo, la sucursal sigue activa y cada producto sigue activo. Devuelve el snapshot del cliente.
func (uc *IssueInvoice) revalidate(ctx context.Context, tx Tx, org uuid.UUID, inv invoice.Invoice) (invoice.CustomerSnapshot, error) {
	c, err := tx.Customers().Get(ctx, org, inv.CustomerID)
	if err != nil {
		return invoice.CustomerSnapshot{}, err
	}
	if !c.IsActive {
		return invoice.CustomerSnapshot{}, ErrCustomerInactive
	}
	b := builder{tx: tx, org: org}
	if err := b.checkBranch(ctx, inv.BranchID); err != nil {
		return invoice.CustomerSnapshot{}, err
	}
	for i, l := range inv.Lines {
		if l.ProductID == nil {
			continue
		}
		p, err := tx.Products().Get(ctx, org, *l.ProductID)
		if err != nil {
			return invoice.CustomerSnapshot{}, err
		}
		if !p.IsActive {
			return invoice.CustomerSnapshot{}, invoice.FieldError{Field: fmt.Sprintf("lines[%d].productId", i),
				Message: "el producto se desactivó después de armar el borrador"}
		}
	}
	return invoice.CustomerSnapshot{
		IdentificationTypeCode: c.Identification.TypeCode, IdentificationNumber: c.Identification.Number,
		LegalName: c.LegalName, Email: c.Email, Phone: c.Phone, Address: c.Address,
	}, nil
}

// assignNumber toma el próximo número visible de la secuencia que corresponde (ADR 0006): candado por (organización,
// tipo), fila bloqueada, avance guardado en esta misma transacción. Si la emisión se revierte, el número también.
func assignNumber(ctx context.Context, tx Tx, org uuid.UUID, inv invoice.Invoice) (string, error) {
	seqs := tx.Sequences()
	docType := string(inv.DocumentType)
	if err := seqs.Lock(ctx, org, docType); err != nil {
		return "", err
	}
	existing, err := seqs.List(ctx, org)
	if err != nil {
		return "", err
	}
	seq := numbering.Choose(org, docType, inv.BranchID, existing)
	if seq.ID != uuid.Nil {
		locked, found, err := seqs.GetForUpdate(ctx, org, seq.Scope)
		if err != nil {
			return "", err
		}
		if found {
			seq = locked
		}
	}
	number, next := seq.Assign()
	if _, err := seqs.Save(ctx, next); err != nil {
		return "", err
	}
	return number, nil
}
