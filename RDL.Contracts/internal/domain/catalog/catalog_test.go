package catalog

import (
	"slices"
	"testing"
)

func validCatalog() ([]Event, map[string]SchemaConsts) {
	var evs []Event
	consts := map[string]SchemaConsts{}
	for _, e := range Architecture62 {
		file := ExpectedSchemaFile(e.Name, 1)
		evs = append(evs, Event{
			Name: e.Name, Version: 1, Producer: e.Producer, Consumers: slices.Clone(e.Consumers),
			SchemaFile: file, Channel: ExpectedChannel(e.Producer, e.Name, 1), Pointer: "/components/messages/" + e.Name,
		})
		consts[file] = SchemaConsts{Found: true, EventType: e.Name, Version: 1, SourceService: e.Producer}
	}
	return evs, consts
}

func rules(vs []Violation) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Rule
	}
	return out
}

func TestKebab(t *testing.T) {
	for in, want := range map[string]string{
		"InvoiceIssued":              "invoice-issued",
		"ElectronicDocumentAccepted": "electronic-document-accepted",
		"CreditNoteIssued":           "credit-note-issued",
	} {
		if got := Kebab(in); got != want {
			t.Errorf("Kebab(%s) = %s", in, got)
		}
	}
}

func TestValidCatalog(t *testing.T) {
	evs, consts := validCatalog()
	if vs := Check(evs, Architecture62, consts); len(vs) != 0 {
		t.Fatalf("violaciones: %+v", vs)
	}
}

func TestViolations(t *testing.T) {
	cases := []struct {
		name   string
		mutate func([]Event, map[string]SchemaConsts) []Event
		rule   string
	}{
		{"evento inventado", func(e []Event, _ map[string]SchemaConsts) []Event {
			return append(e, Event{Name: "OrganizationCreated", Version: 1, Producer: "platform"})
		}, "catalog-unknown-event"},
		{"falta un evento", func(e []Event, _ map[string]SchemaConsts) []Event { return e[1:] }, "catalog-missing-event"},
		{"duplicado", func(e []Event, _ map[string]SchemaConsts) []Event { return append(e, e[0]) }, "catalog-duplicate"},
		{"otro productor", func(e []Event, _ map[string]SchemaConsts) []Event { e[0].Producer = "fiscal"; return e }, "catalog-producer"},
		{"consumidor de más", func(e []Event, _ map[string]SchemaConsts) []Event {
			e[0].Consumers = append(e[0].Consumers, "bff")
			return e
		}, "catalog-consumers"},
		{"schema con otro nombre", func(e []Event, _ map[string]SchemaConsts) []Event {
			e[0].SchemaFile = "schemas/events/InvoiceIssued.json"
			return e
		}, "catalog-schema-file"},
		{"canal con otro nombre", func(e []Event, _ map[string]SchemaConsts) []Event { e[0].Channel = "invoices"; return e }, "catalog-channel"},
		{"schema inexistente", func(e []Event, c map[string]SchemaConsts) []Event {
			delete(c, e[0].SchemaFile)
			return e
		}, "catalog-schema-missing"},
		{"const del schema distinto", func(e []Event, c map[string]SchemaConsts) []Event {
			x := c[e[0].SchemaFile]
			x.SourceService = "receivables"
			c[e[0].SchemaFile] = x
			return e
		}, "catalog-schema-consts"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			evs, consts := validCatalog()
			evs = c.mutate(evs, consts)
			if got := rules(Check(evs, Architecture62, consts)); !slices.Contains(got, c.rule) {
				t.Fatalf("se esperaba %q, se obtuvo %v", c.rule, got)
			}
		})
	}
}
