package events

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"bitbucket.org/rdl/contracts/pkg/events/money"
)

func TestInstantAlwaysUTC(t *testing.T) {
	cr := time.FixedZone("CR", -6*3600)
	b, err := json.Marshal(NewInstant(time.Date(2026, 9, 24, 23, 30, 0, 0, cr)))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `"2026-09-25T05:30:00Z"` {
		t.Fatalf("instante = %s", b)
	}
	var i Instant
	if err := json.Unmarshal([]byte(`"2026-09-24T23:30:00-06:00"`), &i); !errors.Is(err, ErrInvalid) {
		t.Fatalf("un desfase distinto de Z debe rechazarse: %v", err)
	}
	if _, err := json.Marshal(Instant{}); !errors.Is(err, ErrInvalid) {
		t.Fatal("un instante sin valor no se serializa")
	}
}

func TestDateInOrganizationTimezone(t *testing.T) {
	loc, err := time.LoadLocation("America/Costa_Rica")
	if err != nil {
		t.Skip("sin zoneinfo:", err)
	}
	// 05:30 UTC del 25 son las 23:30 del 24 en Costa Rica: la fecha de negocio es el 24.
	d := DateIn(time.Date(2026, 9, 25, 5, 30, 0, 0, time.UTC), loc)
	if d.String() != "2026-09-24" {
		t.Fatalf("fecha = %s", d)
	}
	if _, err := ParseDate("2026-02-30"); !errors.Is(err, ErrInvalid) {
		t.Fatal("día inexistente")
	}
	if !MustDate(t, "2026-09-24").Before(MustDate(t, "2026-10-24")) {
		t.Fatal("Before")
	}
}

func MustDate(t *testing.T, s string) Date {
	t.Helper()
	d, err := ParseDate(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestLineWithoutTaxesSerializesEmptyArray(t *testing.T) {
	b, err := json.Marshal(DocumentLine{LineNumber: 1, Quantity: money.MustQuantity("1")})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"taxes":[]`) {
		t.Fatalf("línea = %s", b)
	}
}

func TestOutboxRow(t *testing.T) {
	org, corr, inv := uuid.New(), uuid.New(), uuid.New()
	now := NewInstant(time.Now())
	today := DateIn(now.Time(), time.UTC)
	e := InvoiceIssuedV1{
		Envelope:  NewEnvelope(InvoiceIssuedType, 1, ServiceBilling, org, corr, now),
		InvoiceID: inv, IssuedAt: now, IssueDate: today, DueDate: today,
		Currency: money.MustCurrency("CRC"), ExchangeRate: money.MustExchangeRate("1"),
	}
	row, err := e.OutboxRow()
	if err != nil {
		t.Fatal(err)
	}
	if row.ID != e.EventID || row.AggregateType != "invoice" || row.AggregateID != inv || row.EventVersion != 1 ||
		row.OrganizationID != org || row.CorrelationID != corr || row.SourceService != ServiceBilling {
		t.Fatalf("fila = %+v", row)
	}
	e.SourceService = ServiceFiscal
	if _, err := e.OutboxRow(); !errors.Is(err, ErrInvalid) {
		t.Fatal("un InvoiceIssued desde fiscal debe rechazarse")
	}
}

// Cada Spec del catálogo coincide con los const de su schema: tipo, versión y productor.
func TestCatalogSpecsMatchSchemaConsts(t *testing.T) {
	for _, s := range Catalog {
		props := loadSchema(t, s.SchemaFile)["properties"].(map[string]any)
		get := func(k string) any { return props[k].(map[string]any)["const"] }
		if get("eventType") != s.Type || get("sourceService") != string(s.Producer) || get("version") != float64(s.Version) {
			t.Errorf("%s: const del schema (%v, %v, %v) ≠ Spec %+v", s.SchemaFile, get("eventType"), get("version"), get("sourceService"), s)
		}
	}
}

func TestValidatorUnknownSchema(t *testing.T) {
	v, err := DefaultValidator()
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Validate("schemas/events/no-existe.v1.json", []byte(`{}`)); err == nil {
		t.Fatal("schema desconocido")
	}
}
