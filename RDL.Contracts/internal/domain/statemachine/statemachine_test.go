package statemachine

import (
	"slices"
	"strings"
	"testing"

	"bitbucket.org/rdl/contracts/internal/domain/catalog"
)

func valid() Machine {
	return Machine{
		Entity: "invoice", Owner: "billing", DocumentTypes: []string{"invoice"},
		Created: Creation{State: "draft", Trigger: Trigger{Command, "POST /v1/invoices"}},
		States: []State{
			{Name: "draft", Description: "borrador"},
			{Name: "issued", Description: "emitida"},
			{Name: "cancelled", Description: "anulada", Final: true},
		},
		Transitions: []Transition{
			{From: "draft", To: "issued", DocumentTypes: []string{"invoice"}, Trigger: Trigger{Command, "POST /v1/invoices/{id}/issue"}, Emits: "InvoiceIssued"},
			{From: "issued", To: "cancelled", Trigger: Trigger{Command, "POST /v1/invoices/{id}/cancel"}, Emits: "InvoiceCancelled"},
		},
		Flags: []Flag{{Name: "requires_correction", SetBy: Trigger{Event, "ElectronicDocumentRejected"}}},
	}
}

func rules(vs []Violation) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Rule
	}
	return out
}

func TestValidMachine(t *testing.T) {
	if vs := valid().Violations(catalog.Architecture62); len(vs) != 0 {
		t.Fatalf("violaciones: %+v", vs)
	}
}

func TestViolations(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Machine)
		rule   string
	}{
		{"estado inalcanzable", func(m *Machine) {
			m.States = append(m.States, State{Name: "archived", Description: "x", Final: true})
		}, "sm-unreachable"},
		{"estado sin salida que no es final", func(m *Machine) { m.States[2].Final = false }, "sm-dead-end"},
		{"final con salida", func(m *Machine) {
			m.Transitions = append(m.Transitions, Transition{From: "cancelled", To: "draft", Trigger: Trigger{Command, "POST /v1/invoices/{id}/reopen"}})
		}, "sm-final-has-exit"},
		{"estado inexistente", func(m *Machine) { m.Transitions[0].To = "sent" }, "sm-unknown-state"},
		{"inicial inexistente", func(m *Machine) { m.Created.State = "new" }, "sm-initial"},
		{"inicial final", func(m *Machine) { m.Created.State = "cancelled" }, "sm-initial"},
		{"evento emitido por quien no lo produce", func(m *Machine) { m.Transitions[0].Emits = "PaymentReceived" }, "sm-emits-foreign-event"},
		{"evento inventado", func(m *Machine) { m.Transitions[0].Emits = "InvoiceApproved" }, "sm-unknown-event"},
		{"reacciona a un evento que no consume", func(m *Machine) {
			m.Transitions[1].Trigger = Trigger{Event, "PaymentReceived"}
		}, "sm-event-not-consumed"},
		{"comando mal escrito", func(m *Machine) { m.Transitions[1].Trigger = Trigger{Command, "cancelar factura"} }, "sm-command"},
		{"disparador desconocido", func(m *Machine) { m.Transitions[1].Trigger = Trigger{"cron", "x"} }, "sm-trigger"},
		{"auto transición", func(m *Machine) { m.Transitions[0].To = "draft" }, "sm-self-loop"},
		{"repetida", func(m *Machine) { m.Transitions = append(m.Transitions, m.Transitions[1]) }, "sm-duplicate-transition"},
		{"tipo de documento ajeno", func(m *Machine) { m.Transitions[0].DocumentTypes = []string{"receipt"} }, "sm-document-type"},
		{"sin descripción", func(m *Machine) { m.States[1].Description = "" }, "sm-description"},
		{"dueño desconocido", func(m *Machine) { m.Owner = "facturacion" }, "sm-owner"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := valid()
			c.mutate(&m)
			if got := rules(m.Violations(catalog.Architecture62)); !slices.Contains(got, c.rule) {
				t.Fatalf("se esperaba %q, se obtuvo %v", c.rule, got)
			}
		})
	}
}

func TestMermaid(t *testing.T) {
	got := valid().Mermaid()
	for _, want := range []string{
		"stateDiagram-v2\n",
		"    [*] --> draft : POST /v1/invoices\n",
		"    draft --> issued : POST /v1/invoices/{id}/issue (invoice) ⇒ InvoiceIssued\n",
		"    cancelled --> [*]\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("falta %q en:\n%s", want, got)
		}
	}
	if got != valid().Mermaid() {
		t.Fatal("el render no es determinista")
	}
}
