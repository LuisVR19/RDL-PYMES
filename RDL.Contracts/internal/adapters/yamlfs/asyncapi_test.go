package yamlfs

import (
	"errors"
	"testing"
	"testing/fstest"

	"bitbucket.org/rdl/contracts/internal/app"
)

func TestAsyncAPILoaderExtractsCatalog(t *testing.T) {
	repo := fstest.MapFS{"asyncapi/a.yaml": {Data: []byte(`
asyncapi: 3.0.0
channels:
  invoiceIssued:
    address: billing.invoice-issued.v1
    messages:
      invoiceIssued:
        $ref: '#/components/messages/InvoiceIssued'
components:
  messages:
    InvoiceIssued:
      name: InvoiceIssued
      payload:
        schema:
          $ref: '../schemas/events/invoice-issued.v1.json'
      x-rdl-version: 1
      x-rdl-producer: billing
      x-rdl-consumers: [fiscal, receivables]
`)}}
	evs, err := AsyncAPILoader{}.Load(repo, "asyncapi/a.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 {
		t.Fatalf("eventos = %+v", evs)
	}
	e := evs[0]
	if e.Name != "InvoiceIssued" || e.Version != 1 || e.Producer != "billing" || len(e.Consumers) != 2 ||
		e.SchemaFile != "schemas/events/invoice-issued.v1.json" || e.Channel != "billing.invoice-issued.v1" ||
		e.Pointer != "/components/messages/InvoiceIssued" {
		t.Fatalf("evento = %+v", e)
	}
}

func TestAsyncAPILoaderErrors(t *testing.T) {
	if _, err := (AsyncAPILoader{}).Load(fstest.MapFS{}, "a.yaml"); !errors.Is(err, app.ErrArtifactMissing) {
		t.Fatalf("err = %v", err)
	}
	bad := fstest.MapFS{"a.yaml": {Data: []byte("channels: [no: es: un mapa")}}
	if _, err := (AsyncAPILoader{}).Load(bad, "a.yaml"); !errors.Is(err, app.ErrArtifactMalformed) {
		t.Fatalf("err = %v", err)
	}
}
