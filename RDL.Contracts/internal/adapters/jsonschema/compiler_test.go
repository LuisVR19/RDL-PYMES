package jsonschema

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

const dialect = `"$schema":"https://json-schema.org/draft/2020-12/schema"`

func id(file string) string { return `"$id":"https://contracts.rdl.invalid/` + file + `"` }

func TestCompileResolvesRelativeRefsAndValidates(t *testing.T) {
	repo := fstest.MapFS{
		"schemas/common/m.json": {Data: []byte(`{` + dialect + `,` + id("schemas/common/m.json") + `,"type":"string","pattern":"^[0-9]+$"}`)},
		"schemas/events/e.json": {Data: []byte(`{` + dialect + `,` + id("schemas/events/e.json") + `,
			"type":"object","additionalProperties":false,"required":["total","at"],
			"properties":{"total":{"$ref":"../common/m.json"},"at":{"type":"string","format":"date-time"}}}`)},
	}
	set, problems, err := Compiler{}.Compile(repo, []string{"schemas/common/m.json", "schemas/events/e.json"})
	if err != nil || len(problems) != 0 {
		t.Fatalf("problems = %+v, err = %v", problems, err)
	}
	var ok any
	_ = json.Unmarshal([]byte(`{"total":"10","at":"2026-09-24T15:04:05Z"}`), &ok)
	if msgs := set.Validate("schemas/events/e.json", ok); len(msgs) != 0 {
		t.Fatalf("válido rechazado: %v", msgs)
	}
	var bad any
	_ = json.Unmarshal([]byte(`{"total":10,"at":"ayer","x":1}`), &bad)
	msgs := set.Validate("schemas/events/e.json", bad)
	joined := strings.Join(msgs, "\n")
	for _, want := range []string{"/total", "/at", "x"} {
		if !strings.Contains(joined, want) {
			t.Errorf("falta %q en:\n%s", want, joined)
		}
	}
}

func TestCompileReportsProblemsPerFile(t *testing.T) {
	repo := fstest.MapFS{
		"schemas/a.json": {Data: []byte(`{` + dialect + `,"$id":"https://otro/a.json","type":"string"}`)},
		"schemas/b.json": {Data: []byte(`{"$schema":"http://json-schema.org/draft-07/schema#",` + id("schemas/b.json") + `}`)},
		"schemas/c.json": {Data: []byte(`{` + dialect + `,` + id("schemas/c.json") + `,"$ref":"https://example.com/x.json"}`)},
		"schemas/d.json": {Data: []byte(`{no es json`)},
	}
	_, problems, err := Compiler{}.Compile(repo, []string{"schemas/a.json", "schemas/b.json", "schemas/c.json", "schemas/d.json"})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range problems {
		got = append(got, p.File+" "+p.Rule)
	}
	for _, want := range []string{"schemas/a.json schema-id", "schemas/b.json schema-dialect", "schemas/c.json schema-compile", "schemas/d.json schema-json"} {
		if !slices.Contains(got, want) {
			t.Errorf("falta %q en %v", want, got)
		}
	}
}
