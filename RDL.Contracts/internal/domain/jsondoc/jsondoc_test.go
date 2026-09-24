package jsondoc

import (
	"encoding/json"
	"testing"
)

func TestRefsAndResolve(t *testing.T) {
	var doc any
	_ = json.Unmarshal([]byte(`{
		"paths": {"/v1/x/{id}": {"get": {"responses": {"200": {"$ref": "#/components/responses/Ok"}}}}},
		"components": {"responses": {"Ok": {"description": "ok"}}, "schemas": {"A": {"$ref": "common.yaml#/components/schemas/Money"}}},
		"list": [{"$ref": "other.json"}]
	}`), &doc)
	refs := Refs(doc)
	want := []Ref{
		{"/components/schemas/A", "common.yaml#/components/schemas/Money"},
		{"/list/0", "other.json"},
		{"/paths/~1v1~1x~1{id}/get/responses/200", "#/components/responses/Ok"},
	}
	if len(refs) != len(want) {
		t.Fatalf("refs = %+v", refs)
	}
	for i := range want {
		if refs[i] != want[i] {
			t.Errorf("refs[%d] = %+v, se esperaba %+v", i, refs[i], want[i])
		}
	}
	if v, ok := Resolve(doc, "/components/responses/Ok/description"); !ok || v != "ok" {
		t.Fatalf("Resolve = %v, %v", v, ok)
	}
	if _, ok := Resolve(doc, "/paths/~1v1~1x~1{id}/get"); !ok {
		t.Fatal("pointer con escape")
	}
	for _, bad := range []string{"/components/responses/Nope", "/list/3", "sin-barra", "/list/x"} {
		if _, ok := Resolve(doc, bad); ok {
			t.Errorf("%q no debería resolver", bad)
		}
	}
	if f, p := Split("common.yaml#/a"); f != "common.yaml" || p != "/a" {
		t.Fatalf("Split = %q %q", f, p)
	}
}
