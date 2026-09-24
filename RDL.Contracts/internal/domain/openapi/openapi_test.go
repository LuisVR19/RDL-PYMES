package openapi

import (
	"encoding/json"
	"testing"
)

func TestOperationsAndViolations(t *testing.T) {
	var doc any
	_ = json.Unmarshal([]byte(`{"paths": {
		"/v1/invoices/{id}/issue": {"parameters": [], "post": {"operationId": "issueInvoice"}},
		"/v1/invoices": {"get": {"operationId": "listInvoices"}, "post": {}},
		"/v1/other": {"get": {"operationId": "listInvoices"}}
	}}`), &doc)
	ops := Operations(doc)
	if len(ops) != 4 {
		t.Fatalf("ops = %+v", ops)
	}
	keys := map[string]bool{}
	for _, o := range ops {
		keys[o.Key()] = true
	}
	if !keys["POST /v1/invoices/{id}/issue"] || !keys["GET /v1/invoices"] {
		t.Fatalf("keys = %v", keys)
	}
	vs := OperationViolations(ops)
	if len(vs) != 2 {
		t.Fatalf("violaciones = %+v", vs)
	}
	for _, v := range vs {
		if v.Rule != "openapi-operation-id" {
			t.Errorf("regla %s", v.Rule)
		}
	}
}
