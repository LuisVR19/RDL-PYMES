package app

import (
	"context"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"bitbucket.org/rdl/contracts/internal/domain/convention"
	"bitbucket.org/rdl/contracts/internal/domain/ownership"
)

func problemsJSON(svc string, codes []string) []byte {
	var items []string
	for _, c := range codes {
		items = append(items, `{"code":"`+c+`","status":400,"title":"t","when":"w"}`)
	}
	return []byte(`{"service":"` + svc + `","problems":[` + strings.Join(items, ",") + `]}`)
}

func TestLintCheck(t *testing.T) {
	repo := fstest.MapFS{
		"schemas/events/x.v1.json":        {Data: []byte(`{"type":"object","properties":{"total":{"type":"number"}}}`)},
		"schemas/events/envelope.v1.json": {Data: []byte(`{"x-rdl-base":true,"type":"object","properties":{"eventId":{"type":"string"}}}`)},
		"openapi/billing.yaml":            {Data: []byte(`{"paths":{"/v1/x":{"post":{}}}}`)},
		"openapi/platform.yaml":           {Data: []byte(`{"paths":{"/v1/y":{"post":{}}}}`)},
	}
	for _, svc := range ownership.Services {
		codes := convention.CommonProblems
		if svc == "fiscal" {
			codes = codes[1:]
		}
		repo["problems/"+svc+".yaml"] = &fstest.MapFile{Data: problemsJSON(svc, codes)}
	}
	delete(repo, "problems/receivables.yaml")
	repo["problems/portal-gateway.yaml"] = &fstest.MapFile{Data: []byte(
		`{"service":"portal-gateway","problems":[{"code":"unauthenticated","status":401,"title":"t","when":"w"},` +
			`{"code":"upstream-error","status":"upstream","title":"t","when":"w"}]}`)}

	got, err := NewLintCheck(jsonDocs{}).Run(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, f := range got {
		keys = append(keys, string(f.Severity)+" "+f.Rule+" "+f.File)
	}
	for _, want := range []string{
		"error no-number schemas/events/x.v1.json",
		"error closed-object schemas/events/x.v1.json",
		"error event-envelope schemas/events/x.v1.json",
		"error openapi-idempotency openapi/billing.yaml",
		"warning openapi-idempotency openapi/platform.yaml",
		"error problem-common problems/fiscal.yaml",
		"error problem-registry problems/receivables.yaml",
		"error problem-common problems/portal-gateway.yaml",
	} {
		if !slices.Contains(keys, want) {
			t.Errorf("falta %q en %v", want, keys)
		}
	}
	for _, k := range keys {
		if strings.Contains(k, "envelope.v1.json") {
			t.Errorf("el sobre es base: %s", k)
		}
	}
}
