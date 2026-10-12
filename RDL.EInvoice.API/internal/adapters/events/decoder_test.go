package events

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	contracts "bitbucket.org/rdl/contracts/pkg/events"

	"rdl/einvoice-api/internal/app"
	"rdl/einvoice-api/internal/platform/examples"
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

func TestDecodeInvoiceIssuedExamples(t *testing.T) {
	d := newDecoder(t)
	valid, invalid := loadExamples(t, "invoice-issued.v1.json")
	for i, payload := range valid {
		ev, err := d.Decode(payload)
		if err != nil {
			t.Fatalf("valid[%d]: %v", i, err)
		}
		cmd, ok := ev.Body.(app.InvoiceIssued)
		if !ok {
			t.Fatalf("valid[%d]: cuerpo %T", i, ev.Body)
		}
		doc := cmd.Document
		if ev.Ref.OrganizationID.String() == "00000000-0000-0000-0000-000000000000" || ev.Ref.Type != "InvoiceIssued" ||
			doc.SourceDocumentNumber == "" || len(doc.Lines) == 0 || doc.Receiver.LegalName == "" {
			t.Errorf("valid[%d]: %+v", i, ev.Ref)
		}
		// El total del evento llega exacto, sin pasar por float.
		var raw struct {
			Total string `json:"total"`
		}
		_ = json.Unmarshal(payload, &raw)
		if !doc.Total.Equal(decimal.RequireFromString(raw.Total)) {
			t.Errorf("valid[%d]: total %s, se esperaba %s", i, doc.Total, raw.Total)
		}
	}
	first := mustDoc(t, d, valid[0]).Document
	if first.Lines[0].Taxes[0].TaxRateCode != "08" || !first.Lines[0].Taxes[0].Amount.Equal(decimal.NewFromInt(1300)) ||
		len(first.PaymentMethods) != 1 || first.PaymentMethods[0].Code != "01" || first.PaymentMethods[0].Amount == nil {
		t.Errorf("valid[0]: línea %+v, medios %+v", first.Lines[0], first.PaymentMethods)
	}
	// Un tipo o versión distintos no tienen consumidor (ErrUnsupportedEvent); el resto no cumple el schema. Los dos
	// son permanentes: dead letter sin reintentos.
	for i, inv := range invalid {
		if _, err := d.Decode(inv.Payload); !errors.Is(err, app.ErrInvalidEvent) && !errors.Is(err, app.ErrUnsupportedEvent) {
			t.Errorf("invalid[%d] (%s): err = %v, se esperaba un error permanente", i, inv.Why, err)
		}
	}
}

func mustDoc(t *testing.T, d *Decoder, payload []byte) app.InvoiceIssued {
	t.Helper()
	ev, err := d.Decode(payload)
	if err != nil {
		t.Fatal(err)
	}
	return ev.Body.(app.InvoiceIssued)
}

// Las notas y la anulación llegan con el incremento 11: hoy van a dead letter como evento sin consumidor.
func TestDecodeUnsupportedAndForeignProducer(t *testing.T) {
	d := newDecoder(t)
	notes, _ := loadExamples(t, "credit-note-issued.v1.json")
	if _, err := d.Decode(notes[0]); !errors.Is(err, app.ErrUnsupportedEvent) {
		t.Errorf("CreditNoteIssued: err = %v, se esperaba ErrUnsupportedEvent", err)
	}
	valid, _ := loadExamples(t, "invoice-issued.v1.json")
	forged := strings.Replace(string(valid[0]), `"sourceService": "billing"`, `"sourceService": "receivables"`, 1)
	forged = strings.Replace(forged, `"sourceService":"billing"`, `"sourceService":"receivables"`, 1)
	if forged == string(valid[0]) {
		t.Skip("el ejemplo no trae sourceService en un formato reconocible")
	}
	if _, err := d.Decode([]byte(forged)); !errors.Is(err, app.ErrInvalidEvent) {
		t.Errorf("InvoiceIssued producido por otro servicio: err = %v", err)
	}
	if _, err := d.Decode([]byte("{no es json")); !errors.Is(err, app.ErrInvalidEvent) {
		t.Errorf("JSON mal formado: err = %v", err)
	}
	if ref := d.Peek([]byte("{no es json")); ref.Type != "unknown" {
		t.Errorf("Peek de basura: %+v", ref)
	}
}
