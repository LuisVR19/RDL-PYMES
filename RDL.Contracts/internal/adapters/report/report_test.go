package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"bitbucket.org/rdl/contracts/internal/app"
	"bitbucket.org/rdl/contracts/internal/domain/finding"
)

func TestTextShowsLocationRuleAndVerdict(t *testing.T) {
	var b bytes.Buffer
	res := app.Result{Findings: []finding.Finding{finding.Errorf("money-as-string", "schemas/a.json", "/properties/total", "usa number")}}
	if err := Write(&b, Text, "lint", res); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	for _, want := range []string{"schemas/a.json#/properties/total", "[money-as-string]", "lint: FALLA (1 errores, 0 avisos)"} {
		if !strings.Contains(out, want) {
			t.Errorf("falta %q en:\n%s", want, out)
		}
	}
}

func TestJSONHasEmptyArrayWhenClean(t *testing.T) {
	var b bytes.Buffer
	if err := Write(&b, JSON, "validate", app.Result{}); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Passed   bool  `json:"passed"`
		Findings []any `json:"findings"`
	}
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Passed || got.Findings == nil {
		t.Fatalf("salida = %s", b.String())
	}
}

func TestParseFormat(t *testing.T) {
	if _, err := ParseFormat("xml"); err == nil {
		t.Fatal("xml no es un formato válido")
	}
}
