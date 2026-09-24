package events

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math/big"
	"os"
	"reflect"
	"testing"

	"bitbucket.org/rdl/contracts/internal/domain/jsonpatch"
)

var repo = os.DirFS("../..")

// examples: cada evento del catálogo con su suite y un constructor vacío del DTO.
var examples = []struct {
	suite string
	spec  Spec
	newE  func() Event
}{
	{"examples/events/invoice-issued.v1.json", InvoiceIssuedSpec, func() Event { return &InvoiceIssuedV1{} }},
	{"examples/events/invoice-cancelled.v1.json", InvoiceCancelledSpec, func() Event { return &InvoiceCancelledV1{} }},
	{"examples/events/credit-note-issued.v1.json", CreditNoteIssuedSpec, func() Event { return &CreditNoteIssuedV1{} }},
	{"examples/events/debit-note-issued.v1.json", DebitNoteIssuedSpec, func() Event { return &DebitNoteIssuedV1{} }},
	{"examples/events/electronic-document-accepted.v1.json", ElectronicDocumentAcceptedSpec, func() Event { return &ElectronicDocumentAcceptedV1{} }},
	{"examples/events/electronic-document-rejected.v1.json", ElectronicDocumentRejectedSpec, func() Event { return &ElectronicDocumentRejectedV1{} }},
	{"examples/events/payment-received.v1.json", PaymentReceivedSpec, func() Event { return &PaymentReceivedV1{} }},
	{"examples/events/receivable-settled.v1.json", ReceivableSettledSpec, func() Event { return &ReceivableSettledV1{} }},
}

type suite struct {
	Schema  string            `json:"schema"`
	Valid   []json.RawMessage `json:"valid"`
	Invalid []struct {
		Value json.RawMessage `json:"value"`
		Base  *int            `json:"base"`
		Patch []jsonpatch.Op  `json:"patch"`
		Why   string          `json:"why"`
	} `json:"invalid"`
}

func loadSuite(t *testing.T, file string) suite {
	t.Helper()
	raw, err := fs.ReadFile(repo, file)
	if err != nil {
		t.Fatal(err)
	}
	var s suite
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

func canonical(t *testing.T, b []byte) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestEveryCatalogEventHasExamples(t *testing.T) {
	if len(examples) != len(Catalog) {
		t.Fatalf("%d suites para %d eventos del catálogo", len(examples), len(Catalog))
	}
	for i, ex := range examples {
		if ex.spec != Catalog[i] || ex.newE().Spec() != ex.spec {
			t.Errorf("%s: el Spec no coincide con el catálogo", ex.suite)
		}
		if s := loadSuite(t, ex.suite); s.Schema != ex.spec.SchemaFile {
			t.Errorf("%s apunta a %s, el Spec a %s", ex.suite, s.Schema, ex.spec.SchemaFile)
		}
	}
}

// Cada ejemplo válido: entra al DTO sin campos desconocidos, sale idéntico, pasa el Validator y sus montos cuadran.
func TestExamplesRoundTrip(t *testing.T) {
	v, err := DefaultValidator()
	if err != nil {
		t.Fatal(err)
	}
	for _, ex := range examples {
		s := loadSuite(t, ex.suite)
		for i, raw := range s.Valid {
			name := fmt.Sprintf("%s valid[%d]", ex.spec.Type, i)
			e := ex.newE()
			dec := json.NewDecoder(bytes.NewReader(raw))
			dec.DisallowUnknownFields()
			if err := dec.Decode(e); err != nil {
				t.Errorf("%s no entra en el DTO: %v", name, err)
				continue
			}
			out, err := json.Marshal(e)
			if err != nil {
				t.Errorf("%s: %v", name, err)
				continue
			}
			if !reflect.DeepEqual(canonical(t, raw), canonical(t, out)) {
				t.Errorf("%s: el round trip cambió el JSON:\nantes:   %s\ndespués: %s", name, raw, out)
			}
			if err := v.Validate(ex.spec.SchemaFile, out); err != nil {
				t.Errorf("%s: el DTO serializado no pasa el schema: %v", name, err)
			}
			if _, err := ToOutboxRow(e); err != nil {
				t.Errorf("%s: %v", name, err)
			}
			checkArithmetic(t, name, e)
		}
	}
}

// El Validator de las APIs rechaza los mismos inválidos que `contractsctl validate`.
func TestValidatorRejectsInvalidExamples(t *testing.T) {
	v, err := DefaultValidator()
	if err != nil {
		t.Fatal(err)
	}
	for _, ex := range examples {
		s := loadSuite(t, ex.suite)
		for i, inv := range s.Invalid {
			payload := []byte(inv.Value)
			if inv.Base != nil {
				doc, err := jsonpatch.Apply(canonical(t, s.Valid[*inv.Base]), inv.Patch)
				if err != nil {
					t.Fatal(err)
				}
				if payload, err = json.Marshal(doc); err != nil {
					t.Fatal(err)
				}
			}
			if err := v.Validate(ex.spec.SchemaFile, payload); !errors.Is(err, ErrInvalid) {
				t.Errorf("%s invalid[%d] (%s) no se rechazó: %v", ex.spec.Type, i, inv.Why, err)
			}
		}
	}
}

type document interface {
	DocumentLines() []DocumentLine
	DocumentTotals() Totals
}

// checkArithmetic aplica las fórmulas de docs/eventos/invoice-issued.md con aritmética racional exacta.
// Los ejemplos están elegidos para no requerir redondeo (la regla es D2, pendiente).
func checkArithmetic(t *testing.T, name string, e Event) {
	t.Helper()
	r := func(s fmt.Stringer) *big.Rat {
		v, ok := new(big.Rat).SetString(s.String())
		if !ok {
			t.Fatalf("%s: %q no es decimal", name, s)
		}
		return v
	}
	eq := func(what string, got, want *big.Rat) {
		if got.Cmp(want) != 0 {
			t.Errorf("%s %s = %s, la fórmula da %s", name, what, got.FloatString(5), want.FloatString(5))
		}
	}
	switch d := e.(type) {
	case document:
		hundred := big.NewRat(100, 1)
		sumSubtotal, sumDiscount, sumTax, sumExo, sumTotal := new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)
		for _, l := range d.DocumentLines() {
			gross := new(big.Rat).Mul(r(l.Quantity), r(l.UnitPrice))
			eq("línea subtotal", r(l.Subtotal), new(big.Rat).Sub(gross, r(l.Discount)))
			lineTax, lineExo := new(big.Rat), new(big.Rat)
			for _, tx := range l.Taxes {
				eq("impuesto", r(tx.Amount), new(big.Rat).Quo(new(big.Rat).Mul(r(tx.TaxableBase), r(tx.Rate)), hundred))
				lineTax.Add(lineTax, r(tx.Amount))
				if x := tx.Exoneration; x != nil {
					eq("exoneración", r(x.Amount), new(big.Rat).Quo(new(big.Rat).Mul(r(tx.Amount), r(x.Percentage)), hundred))
					lineExo.Add(lineExo, r(x.Amount))
				}
			}
			eq("línea tax (neto)", r(l.Tax), new(big.Rat).Sub(lineTax, lineExo))
			eq("línea total", r(l.Total), new(big.Rat).Add(r(l.Subtotal), r(l.Tax)))
			sumSubtotal.Add(sumSubtotal, r(l.Subtotal))
			sumDiscount.Add(sumDiscount, r(l.Discount))
			sumTax.Add(sumTax, lineTax)
			sumExo.Add(sumExo, lineExo)
			sumTotal.Add(sumTotal, r(l.Total))
		}
		tot := d.DocumentTotals()
		eq("subtotal", r(tot.Subtotal), sumSubtotal)
		eq("discount", r(tot.Discount), sumDiscount)
		eq("tax", r(tot.Tax), sumTax)
		eq("exoneration", r(tot.Exoneration), sumExo)
		eq("total", r(tot.Total), new(big.Rat).Sub(new(big.Rat).Add(sumSubtotal, sumTax), sumExo))
		eq("total = Σ líneas", r(tot.Total), sumTotal)
	case *PaymentReceivedV1:
		applied := new(big.Rat)
		for _, a := range d.Applications {
			applied.Add(applied, r(a.Amount))
		}
		if applied.Cmp(r(d.Amount)) > 0 {
			t.Errorf("%s: se aplica %s de un pago de %s", name, applied.FloatString(5), d.Amount)
		}
	}
	switch d := e.(type) {
	case *InvoiceIssuedV1:
		if d.DueDate.Before(d.IssueDate) {
			t.Errorf("%s: dueDate anterior a issueDate", name)
		}
	case *DebitNoteIssuedV1:
		if d.DueDate.Before(d.IssueDate) {
			t.Errorf("%s: dueDate anterior a issueDate", name)
		}
	}
}
