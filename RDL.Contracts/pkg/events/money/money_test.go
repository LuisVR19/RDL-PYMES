package money

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"testing"
)

var repo = os.DirFS("../../..")

// Los patrones de Go y los del JSON Schema tienen que ser idénticos: si alguien cambia uno, falla este test.
func TestPatternsMatchSchemas(t *testing.T) {
	for file, want := range map[string]string{
		"money.json":         AmountPattern,
		"exchange-rate.json": AmountPattern,
		"quantity.json":      QuantityPattern,
		"percentage.json":    PercentagePattern,
		"currency-code.json": CurrencyPattern,
		"tax-rate.json":      TaxRatePattern,
	} {
		raw, err := fs.ReadFile(repo, "schemas/common/"+file)
		if err != nil {
			t.Fatal(err)
		}
		var s struct {
			Pattern string `json:"pattern"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			t.Fatal(err)
		}
		if s.Pattern != want {
			t.Errorf("%s: pattern %q, en Go %q", file, s.Pattern, want)
		}
	}
}

// Los mismos ejemplos que examples/common/*.json: el tipo Go acepta y rechaza lo mismo que el schema.
func TestParseAgreesWithSchemaExamples(t *testing.T) {
	type suite struct {
		Valid   []json.RawMessage `json:"valid"`
		Invalid []struct {
			Value json.RawMessage `json:"value"`
		} `json:"invalid"`
	}
	parsers := map[string]func([]byte) error{
		"money.json":         func(b []byte) error { var v Amount; return json.Unmarshal(b, &v) },
		"exchange-rate.json": func(b []byte) error { var v ExchangeRate; return json.Unmarshal(b, &v) },
		"quantity.json":      func(b []byte) error { var v Quantity; return json.Unmarshal(b, &v) },
		"percentage.json":    func(b []byte) error { var v Percentage; return json.Unmarshal(b, &v) },
		"currency-code.json": func(b []byte) error { var v Currency; return json.Unmarshal(b, &v) },
		"tax-rate.json":      func(b []byte) error { var v TaxRate; return json.Unmarshal(b, &v) },
	}
	for file, parse := range parsers {
		raw, err := fs.ReadFile(repo, "examples/common/"+file)
		if err != nil {
			t.Fatal(err)
		}
		var s suite
		if err := json.Unmarshal(raw, &s); err != nil {
			t.Fatal(err)
		}
		for _, v := range s.Valid {
			if err := parse(v); err != nil {
				t.Errorf("%s: %s debería ser válido: %v", file, v, err)
			}
		}
		for _, inv := range s.Invalid {
			if err := parse(inv.Value); err == nil {
				t.Errorf("%s: %s debería rechazarse", file, inv.Value)
			}
		}
	}
}

func TestAmountRoundTripKeepsTheExactString(t *testing.T) {
	var got struct {
		Total Amount `json:"total"`
	}
	if err := json.Unmarshal([]byte(`{"total":"1300.50"}`), &got); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"total":"1300.50"}` {
		t.Fatalf("round trip = %s", out)
	}
}

func TestZeroValues(t *testing.T) {
	var a Amount
	if a.String() != "0" || !a.IsZero() || MustAmount("0.000").IsZero() == false {
		t.Fatal("el valor cero de Amount es \"0\"")
	}
	if MustAmount("0.01").IsZero() {
		t.Fatal("0.01 no es cero")
	}
	var q Quantity
	if _, err := json.Marshal(q); !errors.Is(err, ErrInvalid) {
		t.Fatalf("una cantidad sin valor no se serializa: %v", err)
	}
	var c Currency
	if _, err := json.Marshal(c); !errors.Is(err, ErrInvalid) {
		t.Fatalf("una moneda sin valor no se serializa: %v", err)
	}
}

func TestRejectsJSONNumber(t *testing.T) {
	var a Amount
	if err := json.Unmarshal([]byte(`11300`), &a); !errors.Is(err, ErrInvalid) {
		t.Fatalf("un number debe rechazarse: %v", err)
	}
}
