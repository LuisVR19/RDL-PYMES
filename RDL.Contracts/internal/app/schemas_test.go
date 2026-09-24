package app

import (
	"context"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"bitbucket.org/rdl/contracts/internal/domain/finding"
)

// fakeSet acepta solo instancias que sean el string "ok".
type fakeSet struct{}

func (fakeSet) Validate(_ string, v any) []string {
	if v == "ok" {
		return nil
	}
	return []string{"/: no es ok"}
}

type fakeCompiler struct {
	problems []finding.Finding
	files    *[]string
}

func (c fakeCompiler) Compile(_ fs.FS, files []string) (SchemaSet, []finding.Finding, error) {
	if c.files != nil {
		*c.files = files
	}
	return fakeSet{}, c.problems, nil
}

func ruleSet(fs []finding.Finding) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Rule + " " + f.Pointer
	}
	return out
}

func TestSchemasCheckCompilesOnlySchemaFiles(t *testing.T) {
	var got []string
	repo := fstest.MapFS{
		"schemas/common/money.json": {Data: []byte("{}")},
		"schemas/events/a.v1.json":  {Data: []byte("{}")},
		"schemas/README.md":         {Data: []byte("x")},
	}
	if _, err := NewSchemasCheck(fakeCompiler{files: &got}).Run(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []string{"schemas/common/money.json", "schemas/events/a.v1.json"}) {
		t.Fatalf("archivos = %v", got)
	}
}

func TestSchemasCheckWithoutSchemas(t *testing.T) {
	fs, _ := NewSchemasCheck(fakeCompiler{}).Run(context.Background(), fstest.MapFS{})
	if len(fs) != 1 || fs[0].Rule != "schemas-missing" {
		t.Fatalf("hallazgos = %+v", fs)
	}
}

func TestExamplesCheck(t *testing.T) {
	repo := fstest.MapFS{
		"schemas/common/x.json": {Data: []byte("{}")},
		"examples/common/x.json": {Data: []byte(`{"schema":"schemas/common/x.json",
			"valid":["ok","mal"],
			"invalid":[{"value":"otro","why":"no es ok"},{"value":"ok","why":"debería fallar"},{"value":"z"}]}`)},
		"examples/common/roto.json":  {Data: []byte(`{"schema":"schemas/common/x.json","extra":1}`)},
		"examples/common/ajeno.json": {Data: []byte(`{"schema":"openapi/x.json","valid":["ok"],"invalid":[{"value":"z","why":"w"}]}`)},
		"examples/common/vacio.json": {Data: []byte(`{"schema":"schemas/common/x.json","valid":["ok"],"invalid":[]}`)},
	}
	fs, err := NewExamplesCheck(fakeCompiler{}).Run(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	got := ruleSet(fs)
	for _, want := range []string{
		"example-valid-rejected /valid/1",
		"example-invalid-accepted /invalid/1",
		"example-invalid-without-reason /invalid/2/why",
		"examples-malformed ",
		"examples-unknown-schema /schema",
		"examples-incomplete ",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("falta %q en %v", want, got)
		}
	}
}

// fakeObjSet acepta objetos con total string.
type fakeObjSet struct{}

func (fakeObjSet) Validate(_ string, v any) []string {
	if m, ok := v.(map[string]any); ok {
		if _, isString := m["total"].(string); isString {
			return nil
		}
	}
	return []string{"/total: no es string"}
}

type fakeObjCompiler struct{}

func (fakeObjCompiler) Compile(fs.FS, []string) (SchemaSet, []finding.Finding, error) {
	return fakeObjSet{}, nil, nil
}

func TestExamplesCheckAppliesPatchesToValidBase(t *testing.T) {
	repo := fstest.MapFS{
		"schemas/e.json": {Data: []byte("{}")},
		"examples/e.json": {Data: []byte(`{"schema":"schemas/e.json","valid":[{"total":"10"}],"invalid":[
			{"base":0,"patch":[{"op":"replace","path":"/total","value":10}],"why":"number"},
			{"base":0,"patch":[{"op":"replace","path":"/total","value":"11"}],"why":"sigue siendo válido"},
			{"base":3,"patch":[{"op":"remove","path":"/total"}],"why":"base inexistente"},
			{"base":0,"value":{"total":1},"patch":[{"op":"remove","path":"/total"}],"why":"ambas formas"}
		]}`)},
	}
	got, err := NewExamplesCheck(fakeObjCompiler{}).Run(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	rules := ruleSet(got)
	for _, want := range []string{
		"example-invalid-accepted /invalid/1",
		"example-invalid-malformed /invalid/2",
		"example-invalid-malformed /invalid/3",
	} {
		if !slices.Contains(rules, want) {
			t.Errorf("falta %q en %v", want, rules)
		}
	}
	if slices.Contains(rules, "example-invalid-accepted /invalid/0") {
		t.Error("el patch con number debió rechazarse")
	}
}

func TestExamplesCheckSkipsWhenSchemasAreBroken(t *testing.T) {
	c := fakeCompiler{problems: []finding.Finding{finding.Errorf("schema-compile", "schemas/x.json", "", "roto")}}
	fs, err := NewExamplesCheck(c).Run(context.Background(), fstest.MapFS{"schemas/x.json": {Data: []byte("{}")}})
	if err != nil || len(fs) != 0 {
		t.Fatalf("hallazgos = %+v, err = %v", fs, err)
	}
}
