package ownership

import (
	"slices"
	"testing"
)

func validMatrix() Matrix {
	var svcs []Service
	for _, n := range Services {
		svcs = append(svcs, Service{Name: n, AppRole: n + "_app", MigratorRole: n + "_migrator"})
	}
	return Matrix{
		Services: svcs,
		Schemas: []Schema{
			{Name: "core", Owner: "platform", Access: []Access{{Service: "billing", ReadAll: true}}},
			{Name: "billing", Owner: "billing"},
			{Name: "fiscal", Owner: "fiscal", Access: []Access{{Service: "billing", ReadTables: []string{"tax_types"}}}},
			{Name: "receivables", Owner: "receivables"},
			{Name: "audit", Owner: SharedOwner, AppendOnly: true, Access: []Access{{Service: "billing", ReadAll: true, Write: []Op{Insert}}}},
		},
	}
}

func rules(vs []Violation) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Rule
	}
	return out
}

func TestValidMatrixHasNoViolations(t *testing.T) {
	if vs := validMatrix().Violations(); len(vs) != 0 {
		t.Fatalf("violaciones inesperadas: %+v", vs)
	}
}

func TestViolations(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Matrix)
		rule   string
	}{
		{"escritura en schema de otra API", func(m *Matrix) {
			m.Schemas[0].Access[0].Write = []Op{Insert}
		}, "ownership-foreign-write"},
		{"update en audit", func(m *Matrix) {
			m.Schemas[4].Access[0].Write = []Op{Insert, Update}
		}, "ownership-append-only"},
		{"delete en audit", func(m *Matrix) {
			m.Schemas[4].Access[0].Write = []Op{Delete}
		}, "ownership-append-only"},
		{"servicio desconocido en access", func(m *Matrix) {
			m.Schemas[0].Access = append(m.Schemas[0].Access, Access{Service: "reports", ReadAll: true})
		}, "ownership-unknown-service"},
		{"dueño desconocido", func(m *Matrix) { m.Schemas[1].Owner = "facturacion" }, "ownership-unknown-owner"},
		{"el dueño aparece en access", func(m *Matrix) {
			m.Schemas[1].Access = []Access{{Service: "billing", ReadAll: true}}
		}, "ownership-owner-in-access"},
		{"read all y tablas a la vez", func(m *Matrix) {
			m.Schemas[2].Access[0].ReadAll = true
		}, "ownership-read-ambiguous"},
		{"rol con otro nombre", func(m *Matrix) { m.Services[1].AppRole = "billing_clerk" }, "ownership-role-name"},
		{"falta un servicio", func(m *Matrix) { m.Services = m.Services[:3] }, "ownership-missing-service"},
		{"servicio sin schema propio", func(m *Matrix) { m.Schemas = m.Schemas[1:] }, "ownership-service-without-schema"},
		{"operación desconocida", func(m *Matrix) {
			m.Schemas[4].Access[0].Write = []Op{"truncate"}
		}, "ownership-unknown-op"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := validMatrix()
			c.mutate(&m)
			if got := rules(m.Violations()); !slices.Contains(got, c.rule) {
				t.Fatalf("se esperaba %q, se obtuvo %v", c.rule, got)
			}
		})
	}
}
