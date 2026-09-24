package app

import (
	"context"
	"encoding/json"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"bitbucket.org/rdl/contracts/internal/domain/finding"
	"bitbucket.org/rdl/contracts/internal/domain/statemachine"
)

// jsonDocs lee los archivos del MapFS como JSON (para no depender del adapter YAML en el test del caso de uso).
type jsonDocs struct{}

func (jsonDocs) Load(repo fs.FS, file string) (any, error) {
	raw, err := fs.ReadFile(repo, file)
	if err != nil {
		return nil, ErrArtifactMissing
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, ErrArtifactMalformed
	}
	return v, nil
}

func TestOpenAPICheck(t *testing.T) {
	repo := fstest.MapFS{
		"openapi/components/common.yaml": {Data: []byte(`{"components":{"parameters":{"Id":{}}}}`)},
		"openapi/billing.yaml": {Data: []byte(`{"paths":{
			"/v1/invoices/{id}/issue":{"post":{"operationId":"issue","parameters":[{"$ref":"components/common.yaml#/components/parameters/Id"}]}},
			"/v1/invoices":{"get":{"operationId":"list","responses":{"200":{"$ref":"#/components/responses/Nope"}}}}
		}}`)},
		"openapi/platform.yaml":       {Data: []byte(`{"paths":{"/healthz":{"get":{}}}}`)},
		"state-machines/invoice.yaml": {Data: []byte("x")},
	}
	machines := fakeMachines{"state-machines/invoice.yaml": {
		Entity: "invoice", Owner: "billing",
		Created: statemachine.Creation{State: "draft", Trigger: statemachine.Trigger{Kind: statemachine.Command, Name: "POST /v1/invoices"}},
		Transitions: []statemachine.Transition{
			{From: "draft", To: "issued", Trigger: statemachine.Trigger{Kind: statemachine.Command, Name: "POST /v1/invoices/{id}/issue"}},
		},
	}}
	got, err := NewOpenAPICheck(jsonDocs{}, fakeSpec{}, machines).Run(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, f := range got {
		keys = append(keys, string(f.Severity)+" "+f.Rule+" "+f.File+"#"+f.Pointer)
	}
	for _, want := range []string{
		"error openapi-ref openapi/billing.yaml#/paths/~1v1~1invoices/get/responses/200",
		"error sm-command-not-in-openapi state-machines/invoice.yaml#/created/trigger",
		"warning openapi-operation-id openapi/platform.yaml#/paths/~1healthz/get",
	} {
		if !slices.Contains(keys, want) {
			t.Errorf("falta %q en %v", want, keys)
		}
	}
	if slices.ContainsFunc(got, func(f finding.Finding) bool { return f.Pointer == "/transitions/0/trigger" }) {
		t.Error("el comando issue existe y se reportó")
	}
	if slices.ContainsFunc(got, func(f finding.Finding) bool {
		return f.Pointer == "/paths/~1v1~1invoices~1{id}~1issue/post/parameters/0"
	}) {
		t.Error("el $ref a common.yaml resuelve y se reportó")
	}
}
