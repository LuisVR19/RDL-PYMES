// Package yamlfs lee artefactos YAML del repo (ownership, máquinas de estado) y los traduce al modelo del dominio.
package yamlfs

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"slices"

	"gopkg.in/yaml.v3"

	"bitbucket.org/rdl/contracts/internal/app"
	"bitbucket.org/rdl/contracts/internal/domain/ownership"
)

type OwnershipLoader struct{}

type ownershipFile struct {
	Version  int                   `yaml:"version"`
	Services map[string]serviceDoc `yaml:"services"`
	Schemas  map[string]schemaDoc  `yaml:"schemas"`
}

type serviceDoc struct {
	DisplayName  string `yaml:"displayName"`
	AppRole      string `yaml:"appRole"`
	MigratorRole string `yaml:"migratorRole"`
}

type schemaDoc struct {
	Owner      string               `yaml:"owner"`
	Purpose    string               `yaml:"purpose"`
	AppendOnly bool                 `yaml:"appendOnly"`
	Access     map[string]accessDoc `yaml:"access"`
}

type accessDoc struct {
	Read  readDoc  `yaml:"read"`
	Write []string `yaml:"write"`
	Note  string   `yaml:"note"`
}

// readDoc acepta `read: all` o `read: [tabla, ...]`.
type readDoc struct {
	all    bool
	tables []string
}

func (r *readDoc) UnmarshalYAML(n *yaml.Node) error {
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Value != "all" {
			return fmt.Errorf("línea %d: read debe ser `all` o una lista de tablas", n.Line)
		}
		r.all = true
		return nil
	case yaml.SequenceNode:
		return n.Decode(&r.tables)
	}
	return fmt.Errorf("línea %d: read debe ser `all` o una lista de tablas", n.Line)
}

func (OwnershipLoader) Load(repo fs.FS, path string) (ownership.Matrix, error) {
	raw, err := fs.ReadFile(repo, path)
	if errors.Is(err, fs.ErrNotExist) {
		return ownership.Matrix{}, app.ErrArtifactMissing
	}
	if err != nil {
		return ownership.Matrix{}, fmt.Errorf("leyendo %s: %w", path, err)
	}
	var doc ownershipFile
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true) // una clave mal escrita (`ower:`) no debe pasar en silencio
	if err := dec.Decode(&doc); err != nil {
		return ownership.Matrix{}, fmt.Errorf("%w: %w", app.ErrArtifactMalformed, err)
	}
	if doc.Version != 1 {
		return ownership.Matrix{}, fmt.Errorf("%w: version %d no soportada (se espera 1)", app.ErrArtifactMalformed, doc.Version)
	}
	return toMatrix(doc), nil
}

func toMatrix(doc ownershipFile) ownership.Matrix {
	var m ownership.Matrix
	for _, name := range slices.Sorted(maps.Keys(doc.Services)) {
		s := doc.Services[name]
		m.Services = append(m.Services, ownership.Service{
			Name: name, DisplayName: s.DisplayName, AppRole: s.AppRole, MigratorRole: s.MigratorRole,
		})
	}
	for _, name := range slices.Sorted(maps.Keys(doc.Schemas)) {
		sc := doc.Schemas[name]
		schema := ownership.Schema{Name: name, Owner: sc.Owner, Purpose: sc.Purpose, AppendOnly: sc.AppendOnly}
		for _, svc := range slices.Sorted(maps.Keys(sc.Access)) {
			a := sc.Access[svc]
			ops := make([]ownership.Op, len(a.Write))
			for i, w := range a.Write {
				ops[i] = ownership.Op(w)
			}
			schema.Access = append(schema.Access, ownership.Access{
				Service: svc, ReadAll: a.Read.all, ReadTables: a.Read.tables, Write: ops, Note: a.Note,
			})
		}
		m.Schemas = append(m.Schemas, schema)
	}
	return m
}
