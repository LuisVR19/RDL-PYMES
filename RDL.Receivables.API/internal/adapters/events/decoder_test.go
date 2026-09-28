package events

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	contracts "bitbucket.org/rdl/contracts/pkg/events"

	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/internal/platform/examples"
)

// examplesDir es el directorio del replace de go.mod: los mismos ejemplos que valida el repo de contratos.
const examplesDir = "../../../../RDL.Contracts/examples/events"

func loadExamples(t *testing.T, name string) ([]json.RawMessage, []examples.Invalid) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(examplesDir, name)) //nolint:gosec // G304: ruta fija del repo de contratos
	if err != nil {
		t.Fatalf("no se pudieron leer los ejemplos de contratos: %v", err)
	}
	valid, invalid, err := examples.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return valid, invalid
}

func newDecoder(t *testing.T) *Decoder {
	t.Helper()
	v, err := contracts.DefaultValidator()
	if err != nil {
		t.Fatal(err)
	}
	return NewDecoder(v)
}

// Cada ejemplo válido de InvoiceIssued se traduce con los datos del sobre y de la factura, sin perder decimales.
func TestDecodeValidInvoiceIssuedExamples(t *testing.T) {
	d := newDecoder(t)
	valid, _ := loadExamples(t, "invoice-issued.v1.json")
	if len(valid) == 0 {
		t.Fatal("contratos no trae ejemplos válidos")
	}
	for i, payload := range valid {
		var want struct {
			EventID          uuid.UUID `json:"eventId"`
			OrganizationID   uuid.UUID `json:"organizationId"`
			CorrelationID    uuid.UUID `json:"correlationId"`
			InvoiceID        uuid.UUID `json:"invoiceId"`
			IssueDate        string    `json:"issueDate"`
			DueDate          string    `json:"dueDate"`
			Currency         string    `json:"currency"`
			Total            string    `json:"total"`
			CustomerSnapshot struct {
				CustomerID uuid.UUID `json:"customerId"`
			} `json:"customerSnapshot"`
		}
		if err := json.Unmarshal(payload, &want); err != nil {
			t.Fatal(err)
		}
		ev, err := d.Decode(payload)
		if err != nil {
			t.Fatalf("valid[%d]: %v", i, err)
		}
		cmd, ok := ev.Body.(app.InvoiceIssued)
		if !ok {
			t.Fatalf("valid[%d]: cuerpo %T", i, ev.Body)
		}
		if ev.Ref.ID != want.EventID || ev.Ref.OrganizationID != want.OrganizationID || ev.Ref.CorrelationID != want.CorrelationID ||
			ev.Ref.Type != contracts.InvoiceIssuedType || ev.Ref.Version != 1 || ev.Ref.OccurredAt.IsZero() {
			t.Errorf("valid[%d]: sobre %+v", i, ev.Ref)
		}
		if cmd.InvoiceID != want.InvoiceID || cmd.CustomerID != want.CustomerSnapshot.CustomerID ||
			cmd.Currency != want.Currency || !cmd.Total.Equal(decimal.RequireFromString(want.Total)) ||
			cmd.IssueDate.String() != want.IssueDate || cmd.DueDate.String() != want.DueDate {
			t.Errorf("valid[%d]: %+v", i, cmd)
		}
		if p := d.Peek(payload); p.ID != want.EventID || p.OrganizationID != want.OrganizationID {
			t.Errorf("valid[%d]: Peek %+v", i, p)
		}
	}
}

// Cada ejemplo inválido de contratos es un error permanente (va a dead letter sin reintentos).
func TestDecodeInvalidInvoiceIssuedExamples(t *testing.T) {
	d := newDecoder(t)
	_, invalid := loadExamples(t, "invoice-issued.v1.json")
	if len(invalid) == 0 {
		t.Fatal("contratos no trae ejemplos inválidos")
	}
	for i, inv := range invalid {
		_, err := d.Decode(inv.Payload)
		if !errors.Is(err, app.ErrInvalidEvent) && !errors.Is(err, app.ErrUnsupportedEvent) {
			t.Errorf("invalid[%d] (%s): %v", i, inv.Why, err)
		}
	}
}

func TestDecodeRejectsWhatReceivablesDoesNotConsume(t *testing.T) {
	d := newDecoder(t)
	// Receivables produce PaymentReceived, no lo consume; los documentos electrónicos son de Fiscal.
	for _, name := range []string{"payment-received.v1.json", "electronic-document-accepted.v1.json"} {
		valid, _ := loadExamples(t, name)
		if _, err := d.Decode(valid[0]); !errors.Is(err, app.ErrUnsupportedEvent) {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, payload := range map[string]string{
		"no es JSON":      `{"eventId":`,
		"sin sobre":       `{}`,
		"versión 2":       `{"eventType":"InvoiceIssued","version":2}`,
		"arreglo":         `[1,2]`,
		"cadena de texto": `"InvoiceIssued"`,
	} {
		_, err := d.Decode([]byte(payload))
		if !errors.Is(err, app.ErrInvalidEvent) && !errors.Is(err, app.ErrUnsupportedEvent) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if ref := d.Peek([]byte("basura")); ref.Type != "unknown" || ref.ID != uuid.Nil {
		t.Errorf("Peek de basura: %+v", ref)
	}
}

// El productor de InvoiceIssued es billing: el mismo evento firmado por otro servicio no se acepta.
func TestDecodeRejectsWrongProducer(t *testing.T) {
	d := newDecoder(t)
	valid, _ := loadExamples(t, "invoice-issued.v1.json")
	var doc map[string]any
	if err := json.Unmarshal(valid[0], &doc); err != nil {
		t.Fatal(err)
	}
	doc["sourceService"] = "fiscal"
	payload, _ := json.Marshal(doc)
	if _, err := d.Decode(payload); !errors.Is(err, app.ErrInvalidEvent) {
		t.Errorf("sourceService fiscal: %v", err)
	}
}

// De punta a punta con el decodificador real: los ejemplos válidos crean una cuenta cada uno y reprocesarlos no
// duplica nada.
func TestExamplesEndToEndAreIdempotent(t *testing.T) {
	d := newDecoder(t)
	store := &memoryStore{inbox: map[uuid.UUID]bool{}, byInvoice: map[[2]uuid.UUID]app.NewReceivable{}}
	handle := app.NewHandleEvent(d, store, map[string]app.EventHandler{
		contracts.InvoiceIssuedType: app.NewCreateReceivableFromInvoice(),
	})
	valid, _ := loadExamples(t, "invoice-issued.v1.json")
	for round := range 2 {
		for i, payload := range valid {
			out, err := handle.Handle(t.Context(), payload)
			want := app.OutcomeProcessed
			if round == 1 {
				want = app.OutcomeDuplicate
			}
			if err != nil || out != want {
				t.Fatalf("ronda %d, valid[%d]: %v, %v", round, i, out, err)
			}
		}
	}
	if len(store.byInvoice) != len(valid) || store.audits != len(valid) {
		t.Errorf("%d cuentas y %d auditorías para %d eventos", len(store.byInvoice), store.audits, len(valid))
	}
}

// memoryStore es una base mínima en memoria para el test de punta a punta.
type memoryStore struct {
	inbox     map[uuid.UUID]bool
	byInvoice map[[2]uuid.UUID]app.NewReceivable
	audits    int
}

func (m *memoryStore) WithinServiceTx(ctx context.Context, _ uuid.UUID, fn func(context.Context, app.EventTx) error) error {
	return fn(ctx, memoryTx{m: m})
}

// memoryTx embebe app.EventTx: InvoiceIssued no usa Ledger ni Outbox.
type memoryTx struct {
	app.EventTx
	m *memoryStore
}

func (t memoryTx) Inbox() app.Inbox                     { return t }
func (t memoryTx) ReceivableStore() app.ReceivableStore { return t }
func (t memoryTx) Audit() app.AuditRecorder             { return t }

func (t memoryTx) Claim(_ context.Context, ref app.EventRef) (bool, error) {
	if t.m.inbox[ref.ID] {
		return false, nil
	}
	t.m.inbox[ref.ID] = true
	return true, nil
}

func (t memoryTx) MarkProcessed(context.Context, app.EventRef) error { return nil }

func (t memoryTx) FindByInvoice(_ context.Context, org, invoiceID uuid.UUID) (app.ReceivableRecord, bool, error) {
	r, ok := t.m.byInvoice[[2]uuid.UUID{org, invoiceID}]
	if !ok {
		return app.ReceivableRecord{}, false, nil
	}
	inv := r.Receivable.Invoice()
	return app.ReceivableRecord{ID: r.Receivable.ID(), CustomerID: inv.CustomerID, Currency: inv.Currency, Original: inv.Original}, true, nil
}

func (t memoryTx) Create(_ context.Context, org uuid.UUID, r app.NewReceivable) error {
	t.m.byInvoice[[2]uuid.UUID{org, r.Receivable.Invoice().ID}] = r
	return nil
}

func (t memoryTx) Record(context.Context, app.AuditEvent) error {
	t.m.audits++
	return nil
}

// Notas y anulación: cada ejemplo válido de contratos se traduce a su comando y cada inválido es permanente.
func TestDecodeNoteAndCancellationExamples(t *testing.T) {
	d := newDecoder(t)
	for file, check := range map[string]func(t *testing.T, body any, raw map[string]any){
		"credit-note-issued.v1.json": func(t *testing.T, body any, raw map[string]any) {
			c, ok := body.(app.CreditNoteIssued)
			if !ok || c.InvoiceID.String() != raw["referencedInvoiceId"] || c.DocumentID.String() != raw["documentId"] ||
				!c.Total.Equal(decimal.RequireFromString(raw["total"].(string))) || c.Reason != raw["referenceReason"] {
				t.Errorf("%+v", body)
			}
		},
		"debit-note-issued.v1.json": func(t *testing.T, body any, raw map[string]any) {
			c, ok := body.(app.DebitNoteIssued)
			if !ok || c.InvoiceID.String() != raw["referencedInvoiceId"] || c.DueDate.String() != raw["dueDate"] ||
				!c.Total.Equal(decimal.RequireFromString(raw["total"].(string))) {
				t.Errorf("%+v", body)
			}
		},
		"invoice-cancelled.v1.json": func(t *testing.T, body any, raw map[string]any) {
			c, ok := body.(app.InvoiceCancelled)
			if !ok || c.InvoiceID.String() != raw["invoiceId"] || c.Reason != raw["reason"] || c.Currency != raw["currency"] {
				t.Errorf("%+v", body)
			}
		},
	} {
		t.Run(file, func(t *testing.T) {
			valid, invalid := loadExamples(t, file)
			for i, payload := range valid {
				ev, err := d.Decode(payload)
				if err != nil {
					t.Fatalf("valid[%d]: %v", i, err)
				}
				var raw map[string]any
				_ = json.Unmarshal(payload, &raw)
				check(t, ev.Body, raw)
			}
			for i, inv := range invalid {
				if _, err := d.Decode(inv.Payload); !errors.Is(err, app.ErrInvalidEvent) && !errors.Is(err, app.ErrUnsupportedEvent) {
					t.Errorf("invalid[%d] (%s): %v", i, inv.Why, err)
				}
			}
		})
	}
}
