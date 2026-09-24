package jsonschema

import (
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"bitbucket.org/rdl/contracts/internal/app"
)

const minimalAsyncAPI = `
asyncapi: 3.0.0
info:
  title: t
  version: 0.1.0
channels:
  c:
    address: a.b.v1
    messages:
      m:
        $ref: '#/components/messages/M'
operations:
  send:
    action: send
    channel:
      $ref: '#/channels/c'
components:
  messages:
    M:
      payload:
        schemaFormat: application/schema+json;version=draft-2020-12
        schema:
          $ref: '../schemas/x.json'
`

func TestAsyncAPIValidatorAcceptsMinimalDocument(t *testing.T) {
	fs, err := AsyncAPI30().Validate(fstest.MapFS{"a.yaml": {Data: []byte(minimalAsyncAPI)}}, "a.yaml")
	if err != nil || len(fs) != 0 {
		t.Fatalf("hallazgos = %+v, err = %v", fs, err)
	}
}

func TestAsyncAPIValidatorReportsSpecErrors(t *testing.T) {
	doc := strings.Replace(minimalAsyncAPI, "action: send", "action: publish", 1)
	doc = strings.Replace(doc, "  version: 0.1.0\n", "", 1)
	fs, err := AsyncAPI30().Validate(fstest.MapFS{"a.yaml": {Data: []byte(doc)}}, "a.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var all []string
	for _, f := range fs {
		all = append(all, f.Message)
	}
	joined := strings.Join(all, "\n")
	for _, want := range []string{"/operations/send/action", "/info"} {
		if !strings.Contains(joined, want) {
			t.Errorf("falta %q en:\n%s", want, joined)
		}
	}
}

func TestAsyncAPIValidatorMissingFile(t *testing.T) {
	if _, err := AsyncAPI30().Validate(fstest.MapFS{}, "a.yaml"); !errors.Is(err, app.ErrArtifactMissing) {
		t.Fatalf("err = %v", err)
	}
}
