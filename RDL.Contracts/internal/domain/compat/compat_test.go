package compat

import (
	"encoding/json"
	"slices"
	"testing"
)

func doc(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

const base = `{
  "type": "object", "unevaluatedProperties": false, "required": ["id", "total"],
  "properties": {
    "id": {"$ref": "uuid.json"},
    "total": {"$ref": "money.json"},
    "status": {"type": "string", "enum": ["open", "paid"]},
    "reason": {"type": "string", "maxLength": 500},
    "lines": {"type": "array", "items": {"type": "object", "properties": {"n": {"type": "integer", "minimum": 1}}}}
  }
}`

func rules(cs []Change) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Rule
	}
	return out
}

func TestIdenticalIsCompatible(t *testing.T) {
	if cs := Schemas(doc(t, base), doc(t, base)); len(cs) != 0 {
		t.Fatalf("cambios = %+v", cs)
	}
}

func TestCompatibleChanges(t *testing.T) {
	cases := map[string]string{
		"agregar campo opcional": `"properties": {"id": {"$ref": "uuid.json"}, "total": {"$ref": "money.json"}, "status": {"type": "string", "enum": ["open", "paid"]}, "reason": {"type": "string", "maxLength": 500}, "lines": {"type": "array", "items": {"type": "object", "properties": {"n": {"type": "integer", "minimum": 1}}}}, "notes": {"type": "string"}}`,
		"aflojar maxLength":      `"properties": {"id": {"$ref": "uuid.json"}, "total": {"$ref": "money.json"}, "status": {"type": "string", "enum": ["open", "paid"]}, "reason": {"type": "string", "maxLength": 1000}, "lines": {"type": "array", "items": {"type": "object", "properties": {"n": {"type": "integer", "minimum": 1}}}}}`,
		"aflojar minimum":        `"properties": {"id": {"$ref": "uuid.json"}, "total": {"$ref": "money.json"}, "status": {"type": "string", "enum": ["open", "paid"]}, "reason": {"type": "string", "maxLength": 500}, "lines": {"type": "array", "items": {"type": "object", "properties": {"n": {"type": "integer", "minimum": 0}}}}}`,
	}
	for name, props := range cases {
		cur := `{"type": "object", "unevaluatedProperties": false, "required": ["id", "total"], ` + props + `}`
		if cs := Schemas(doc(t, base), doc(t, cur)); len(cs) != 0 {
			t.Errorf("%s: se marcó incompatible: %+v", name, cs)
		}
	}
}

func TestBreakingChanges(t *testing.T) {
	mutate := func(f func(m map[string]any)) any {
		v := doc(t, base)
		f(v.(map[string]any))
		return v
	}
	props := func(m map[string]any) map[string]any { return m["properties"].(map[string]any) }
	cases := []struct {
		name string
		cur  any
		rule string
	}{
		{"quitar campo", mutate(func(m map[string]any) { delete(props(m), "reason") }), "property-removed"},
		{"campo nuevo obligatorio", mutate(func(m map[string]any) { m["required"] = []any{"id", "total", "status"} }), "required-added"},
		{"cambiar tipo", mutate(func(m map[string]any) { props(m)["reason"] = map[string]any{"type": "integer", "maxLength": 500.0} }), "type-changed"},
		{"cambiar ref (money → number)", mutate(func(m map[string]any) { props(m)["total"] = map[string]any{"type": "number"} }), "$ref-changed"},
		{"quitar valor de enum", mutate(func(m map[string]any) { props(m)["status"].(map[string]any)["enum"] = []any{"open"} }), "enum-value-removed"},
		{"agregar valor a enum", mutate(func(m map[string]any) {
			props(m)["status"].(map[string]any)["enum"] = []any{"open", "paid", "void"}
		}), "enum-value-added"},
		{"endurecer maxLength", mutate(func(m map[string]any) { props(m)["reason"].(map[string]any)["maxLength"] = 100.0 }), "bound-tightened"},
		{"límite nuevo", mutate(func(m map[string]any) { props(m)["reason"].(map[string]any)["minLength"] = 1.0 }), "bound-tightened"},
		{"cambiar pattern", mutate(func(m map[string]any) { props(m)["reason"].(map[string]any)["pattern"] = "^x$" }), "pattern-changed"},
		{"abrir el objeto", mutate(func(m map[string]any) { delete(m, "unevaluatedProperties") }), "unevaluatedProperties-changed"},
		{"anidado en items", mutate(func(m map[string]any) {
			props(m)["lines"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)["n"].(map[string]any)["minimum"] = 2.0
		}), "bound-tightened"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rules(Schemas(doc(t, base), c.cur)); !slices.Contains(got, c.rule) {
				t.Fatalf("se esperaba %q, se obtuvo %v", c.rule, got)
			}
		})
	}
}

func TestOperations(t *testing.T) {
	old := []Operation{
		{Key: "GET /v1/x", Params: []Param{{Name: "limit", In: "query"}}},
		{Key: "POST /v1/x", Params: []Param{{Name: "Idempotency-Key", In: "header", Required: true}}},
		{Key: "DELETE /v1/x/{id}"},
	}
	cur := []Operation{
		{Key: "GET /v1/x", Params: []Param{{Name: "limit", In: "query"}, {Name: "status", In: "query", Required: true}}},
		{Key: "POST /v1/x", Params: []Param{{Name: "Idempotency-Key", In: "header", Required: true}}, BodyRequire: true},
		{Key: "PUT /v1/x/{id}"},
	}
	got := rules(Operations(old, cur))
	for _, want := range []string{"operation-removed", "parameter-required-added", "body-required-added"} {
		if !slices.Contains(got, want) {
			t.Errorf("falta %q en %v", want, got)
		}
	}
	if len(got) != 3 {
		t.Errorf("cambios = %v (agregar PUT es compatible)", got)
	}
}
