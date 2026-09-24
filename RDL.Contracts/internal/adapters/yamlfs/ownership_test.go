package yamlfs

import (
	"errors"
	"testing"
	"testing/fstest"

	"bitbucket.org/rdl/contracts/internal/app"
)

func TestLoadOwnershipMapsReadForms(t *testing.T) {
	repo := fstest.MapFS{"o.yaml": {Data: []byte(`
version: 1
services:
  billing: {displayName: Billing API, appRole: billing_app, migratorRole: billing_migrator}
schemas:
  fiscal:
    owner: fiscal
    access:
      billing: {read: [tax_types, tax_rates]}
  core:
    owner: platform
    access:
      billing: {read: all}
  audit:
    owner: database-platform
    appendOnly: true
    access:
      billing: {read: all, write: [insert]}
`)}}
	m, err := OwnershipLoader{}.Load(repo, "o.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Schemas) != 3 || m.Schemas[0].Name != "audit" {
		t.Fatalf("schemas = %+v", m.Schemas)
	}
	audit, core, fiscal := m.Schemas[0], m.Schemas[1], m.Schemas[2]
	if !audit.AppendOnly || audit.Access[0].Write[0] != "insert" {
		t.Errorf("audit = %+v", audit)
	}
	if !core.Access[0].ReadAll {
		t.Errorf("core: read all no se interpretó: %+v", core.Access[0])
	}
	if fiscal.Access[0].ReadAll || len(fiscal.Access[0].ReadTables) != 2 {
		t.Errorf("fiscal: tablas no se interpretaron: %+v", fiscal.Access[0])
	}
}

func TestLoadOwnershipErrors(t *testing.T) {
	cases := map[string]struct {
		data string
		want error
	}{
		"clave desconocida": {"version: 1\nschemas:\n  core: {ower: platform}\n", app.ErrArtifactMalformed},
		"read inválido":     {"version: 1\nschemas:\n  core:\n    owner: platform\n    access:\n      billing: {read: some}\n", app.ErrArtifactMalformed},
		"versión":           {"version: 2\n", app.ErrArtifactMalformed},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := OwnershipLoader{}.Load(fstest.MapFS{"o.yaml": {Data: []byte(c.data)}}, "o.yaml")
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v", err)
			}
		})
	}
	if _, err := (OwnershipLoader{}).Load(fstest.MapFS{}, "o.yaml"); !errors.Is(err, app.ErrArtifactMissing) {
		t.Fatalf("archivo inexistente: err = %v", err)
	}
}
