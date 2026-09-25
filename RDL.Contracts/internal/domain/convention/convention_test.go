package convention

import (
	"encoding/json"
	"slices"
	"testing"
)

func doc(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func rules(vs []Violation) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Rule + " " + v.Pointer
	}
	return out
}

func TestSchemaRules(t *testing.T) {
	d := doc(t, `{"type":"object","additionalProperties":false,"properties":{
		"total":{"type":"number"},
		"lineCount":{"type":"integer"},
		"taxAmount":{"type":"integer"},
		"issued_at":{"$ref":"utc-datetime.json"},
		"paidAt":{"type":"string"},
		"createdAt":{"type":"string","format":"date-time"},
		"dueDate":{"$ref":"business-date.json"},
		"receivedOn":{"type":"string"},
		"eventType":{"const":"X"},
		"lines":{"type":"array","items":{"type":"object","properties":{"unitPrice":{"type":"number"}}}}
	}}`)
	got := rules(SchemaRules(d, ""))
	want := []string{
		"no-number /properties/total",
		"decimal-as-integer /properties/taxAmount",
		"camel-case /properties/issued_at",
		"instant-format /properties/paidAt",
		"date-format /properties/receivedOn",
		"no-number /properties/lines/items/properties/unitPrice",
	}
	for _, w := range want {
		if !slices.Contains(got, w) {
			t.Errorf("falta %q en %v", w, got)
		}
	}
	if len(got) != len(want) {
		t.Errorf("violaciones de más: %v", got)
	}
}

func TestClosedObjects(t *testing.T) {
	open := doc(t, `{"type":"object","properties":{"a":{"type":"object","properties":{"b":{"type":"string"}}}},
		"if":{"properties":{"a":{"type":"string"}}}}`)
	if got := rules(ClosedObjects(open)); len(got) != 2 {
		t.Fatalf("se esperaban la raíz y /properties/a (no el if): %v", got)
	}
	base := doc(t, `{"x-rdl-base":true,"type":"object","properties":{"a":{"type":"string"}}}`)
	if got := ClosedObjects(base); len(got) != 0 {
		t.Fatalf("un schema base puede ser abierto: %v", got)
	}
}

func TestEventRules(t *testing.T) {
	ok := doc(t, `{"allOf":[{"$ref":"envelope.v1.json"}],"unevaluatedProperties":false,
		"properties":{"eventType":{"const":"A"},"version":{"const":1},"sourceService":{"const":"billing"}}}`)
	if got := EventRules(ok); len(got) != 0 {
		t.Fatalf("violaciones: %v", got)
	}
	bad := doc(t, `{"properties":{"eventType":{"type":"string"}}}`)
	got := rules(EventRules(bad))
	for _, w := range []string{"event-envelope /allOf", "event-closed /unevaluatedProperties", "event-const /properties/eventType", "event-const /properties/version"} {
		if !slices.Contains(got, w) {
			t.Errorf("falta %q en %v", w, got)
		}
	}
}

func TestOpenAPIRules(t *testing.T) {
	d := doc(t, `{"paths":{
		"/v1/x":{"post":{"parameters":[{"$ref":"common.yaml#/components/parameters/IdempotencyKey"}]},
		         "get":{"parameters":[{"name":"organizationId","in":"query"}]}},
		"/v1/y":{"post":{}},
		"/internal/v1/z":{"post":{}},
		"/x":{"get":{}}
	}}`)
	got := rules(OpenAPIRules(d))
	for _, w := range []string{
		"openapi-tenant-param /paths/~1v1~1x/get/parameters/0",
		"openapi-idempotency /paths/~1v1~1y/post",
		"openapi-path-version /paths/~1x",
	} {
		if !slices.Contains(got, w) {
			t.Errorf("falta %q en %v", w, got)
		}
	}
	if len(got) != 3 {
		t.Errorf("violaciones de más (el POST interno no exige clave): %v", got)
	}
}

// Un servicio de borde tiene su propio mínimo y es el único que puede conservar el status de otra API.
func TestProblemRulesForEdgeServices(t *testing.T) {
	var ps []Problem
	for _, c := range EdgeProblems["portal-gateway"] {
		ps = append(ps, Problem{Code: c, Status: 404, Title: "t", When: "w"})
	}
	ps = append(ps, Problem{Code: "upstream-error", UpstreamStatus: true, Title: "t", When: "w"})
	if got := ProblemRules("portal-gateway", ps); len(got) != 0 {
		t.Fatalf("violaciones: %v", got)
	}
	if got := rules(ProblemRules("portal-gateway", ps[1:])); !slices.Contains(got, "problem-common /problems") {
		t.Errorf("sin unauthenticated debería fallar: %v", got)
	}

	var domain []Problem
	for _, c := range CommonProblems {
		domain = append(domain, Problem{Code: c, Status: 400, Title: "t", When: "w"})
	}
	domain = append(domain, Problem{Code: "upstream-error", UpstreamStatus: true, Title: "t", When: "w"})
	if got := rules(ProblemRules("billing", domain)); !slices.Contains(got, "problem-status /problems/11/status") {
		t.Errorf("una API de dominio no conserva status ajenos: %v", got)
	}
}

func TestProblemRules(t *testing.T) {
	var ps []Problem
	for _, c := range CommonProblems {
		ps = append(ps, Problem{Code: c, Status: 400, Title: "t", When: "w"})
	}
	if got := ProblemRules("billing", ps); len(got) != 0 {
		t.Fatalf("violaciones: %v", got)
	}
	ps = append(ps[1:], Problem{Code: "Bad_Code", Status: 200, Title: "t", When: "w"}, Problem{Code: "conflict", Status: 409})
	got := rules(ProblemRules("billing", ps))
	for _, w := range []string{"problem-code /problems/10/code", "problem-status /problems/10/status", "problem-duplicate /problems/11/code", "problem-docs /problems/11", "problem-common /problems"} {
		if !slices.Contains(got, w) {
			t.Errorf("falta %q en %v", w, got)
		}
	}
}
