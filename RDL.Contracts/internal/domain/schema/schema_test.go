package schema

import "testing"

func TestExpectedIDRoundTrip(t *testing.T) {
	id := ExpectedID("schemas/common/money.json")
	if id != "https://contracts.rdl.invalid/schemas/common/money.json" {
		t.Fatalf("id = %s", id)
	}
	if f, ok := FileForID(id); !ok || f != "schemas/common/money.json" {
		t.Fatalf("FileForID = %q, %v", f, ok)
	}
	if _, ok := FileForID("https://json-schema.org/draft/2020-12/schema"); ok {
		t.Fatal("un id ajeno no pertenece al repo")
	}
}

func TestIsSchemaFile(t *testing.T) {
	for f, want := range map[string]bool{
		"schemas/common/money.json":  true,
		"schemas/events/x.v1.json":   true,
		"examples/common/money.json": false,
		"schemas/common/README.md":   false,
		"openapi/components/x.json":  false,
	} {
		if IsSchemaFile(f) != want {
			t.Errorf("IsSchemaFile(%q) != %v", f, want)
		}
	}
}
