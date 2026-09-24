package app

import (
	"context"
	"errors"
	"slices"
	"testing"
	"testing/fstest"
)

func TestBreaking(t *testing.T) {
	base := fstest.MapFS{
		"schemas/events/a.v1.json":       {Data: []byte(`{"type":"object","required":["id"],"properties":{"id":{"type":"string"},"note":{"type":"string"}}}`)},
		"schemas/events/b.v1.json":       {Data: []byte(`{"type":"object"}`)},
		"openapi/billing.yaml":           {Data: []byte(`{"paths":{"/v1/x":{"get":{"parameters":[{"$ref":"components/common.yaml#/components/parameters/Limit"}]}},"/v1/y":{"delete":{}}}}`)},
		"openapi/components/common.yaml": {Data: []byte(`{"components":{"parameters":{"Limit":{"name":"limit","in":"query"},"Req":{"name":"q","in":"query","required":true}}}}`)},
		"asyncapi/asyncapi.yaml":         {Data: []byte(`{"components":{"messages":{"A":{},"B":{}}}}`)},
	}
	cur := fstest.MapFS{
		"schemas/events/a.v1.json":       {Data: []byte(`{"type":"object","required":["id","note"],"properties":{"id":{"type":"string"},"note":{"type":"string"},"extra":{"type":"string"}}}`)},
		"openapi/billing.yaml":           {Data: []byte(`{"paths":{"/v1/x":{"get":{"parameters":[{"$ref":"components/common.yaml#/components/parameters/Limit"},{"$ref":"components/common.yaml#/components/parameters/Req"}]}}}}`)},
		"openapi/components/common.yaml": base["openapi/components/common.yaml"],
		"asyncapi/asyncapi.yaml":         {Data: []byte(`{"components":{"messages":{"A":{}}}}`)},
	}
	res, err := NewBreaking(jsonDocs{}).Execute(context.Background(), base, cur)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range res.Findings {
		got = append(got, f.Rule+" "+f.File)
	}
	for _, want := range []string{
		"required-added schemas/events/a.v1.json",
		"schema-file-removed schemas/events/b.v1.json",
		"parameter-required-added openapi/billing.yaml",
		"operation-removed openapi/billing.yaml",
		"event-removed asyncapi/asyncapi.yaml",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("falta %q en %v", want, got)
		}
	}
	if !res.Failed() {
		t.Fatal("los cambios incompatibles deben fallar")
	}
}

func TestBreakingIdenticalAndInvalidBase(t *testing.T) {
	repo := fstest.MapFS{"schemas/common/x.json": {Data: []byte(`{"type":"string"}`)}}
	res, err := NewBreaking(jsonDocs{}).Execute(context.Background(), repo, repo)
	if err != nil || res.Failed() || len(res.Findings) != 0 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if _, err := NewBreaking(jsonDocs{}).Execute(context.Background(), fstest.MapFS{}, repo); !errors.Is(err, ErrInvalidBase) {
		t.Fatalf("err = %v", err)
	}
}
