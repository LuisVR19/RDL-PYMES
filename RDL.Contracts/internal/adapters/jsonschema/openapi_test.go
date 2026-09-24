package jsonschema

import (
	"strings"
	"testing"
	"testing/fstest"
)

const minimalOpenAPI = `
openapi: 3.1.0
info: {title: t, version: 0.1.0}
paths:
  /v1/x:
    get:
      operationId: getX
      responses:
        '200': {description: ok}
`

func TestOpenAPI31AcceptsMinimalDocument(t *testing.T) {
	fs, err := OpenAPI31().Validate(fstest.MapFS{"o.yaml": {Data: []byte(minimalOpenAPI)}}, "o.yaml")
	if err != nil || len(fs) != 0 {
		t.Fatalf("hallazgos = %+v, err = %v", fs, err)
	}
}

func TestOpenAPI31ReportsSpecErrors(t *testing.T) {
	doc := strings.Replace(minimalOpenAPI, "openapi: 3.1.0", "openapi: 3.0.3", 1)
	doc = strings.Replace(doc, "info: {title: t, version: 0.1.0}", "info: {title: t}", 1)
	fs, err := OpenAPI31().Validate(fstest.MapFS{"o.yaml": {Data: []byte(doc)}}, "o.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var all []string
	for _, f := range fs {
		all = append(all, f.Message)
	}
	joined := strings.Join(all, "\n")
	for _, want := range []string{"/openapi", "/info"} {
		if !strings.Contains(joined, want) {
			t.Errorf("falta %q en:\n%s", want, joined)
		}
	}
}
