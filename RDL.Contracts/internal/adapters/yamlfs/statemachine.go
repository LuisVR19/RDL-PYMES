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
	"bitbucket.org/rdl/contracts/internal/domain/statemachine"
)

type StateMachineLoader struct{}

type triggerDoc struct {
	Kind string `yaml:"kind"`
	Name string `yaml:"name"`
}

func (t triggerDoc) toDomain() statemachine.Trigger {
	return statemachine.Trigger{Kind: statemachine.TriggerKind(t.Kind), Name: t.Name}
}

type stateDoc struct {
	Description string `yaml:"description"`
	Final       bool   `yaml:"final"`
}

type machineDoc struct {
	Version       int      `yaml:"version"`
	Entity        string   `yaml:"entity"`
	Owner         string   `yaml:"owner"`
	Table         string   `yaml:"table"`
	Column        string   `yaml:"column"`
	DocumentTypes []string `yaml:"documentTypes"`
	Created       struct {
		State   string       `yaml:"state"`
		Trigger triggerDoc   `yaml:"trigger"`
		AlsoBy  []triggerDoc `yaml:"alsoBy"`
		Emits   string       `yaml:"emits"`
	} `yaml:"created"`
	// States es un nodo para conservar el orden del documento (lo usa el diagrama).
	States      yaml.Node `yaml:"states"`
	Transitions []struct {
		From          string     `yaml:"from"`
		To            string     `yaml:"to"`
		DocumentTypes []string   `yaml:"documentTypes"`
		Trigger       triggerDoc `yaml:"trigger"`
		Emits         string     `yaml:"emits"`
		Requires      []string   `yaml:"requires"`
	} `yaml:"transitions"`
	Flags map[string]struct {
		Description string     `yaml:"description"`
		SetBy       triggerDoc `yaml:"setBy"`
	} `yaml:"flags"`
	TODO []string `yaml:"todo"`
}

func (StateMachineLoader) Load(repo fs.FS, file string) (statemachine.Machine, error) {
	raw, err := fs.ReadFile(repo, file)
	if errors.Is(err, fs.ErrNotExist) {
		return statemachine.Machine{}, app.ErrArtifactMissing
	}
	if err != nil {
		return statemachine.Machine{}, fmt.Errorf("leyendo %s: %w", file, err)
	}
	var doc machineDoc
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return statemachine.Machine{}, fmt.Errorf("%w: %w", app.ErrArtifactMalformed, err)
	}
	if doc.Version != 1 {
		return statemachine.Machine{}, fmt.Errorf("%w: version %d no soportada (se espera 1)", app.ErrArtifactMalformed, doc.Version)
	}

	m := statemachine.Machine{
		Entity: doc.Entity, Owner: doc.Owner, Table: doc.Table, Column: doc.Column, DocumentTypes: doc.DocumentTypes,
		Created: statemachine.Creation{State: doc.Created.State, Trigger: doc.Created.Trigger.toDomain(), Emits: doc.Created.Emits},
		TODO:    doc.TODO,
	}
	for _, t := range doc.Created.AlsoBy {
		m.Created.AlsoBy = append(m.Created.AlsoBy, t.toDomain())
	}
	if doc.States.Kind != 0 && doc.States.Kind != yaml.MappingNode {
		return statemachine.Machine{}, fmt.Errorf("%w: states debe ser un mapa", app.ErrArtifactMalformed)
	}
	for i := 0; i+1 < len(doc.States.Content); i += 2 {
		var s stateDoc
		if err := doc.States.Content[i+1].Decode(&s); err != nil {
			return statemachine.Machine{}, fmt.Errorf("%w: estado %s: %w", app.ErrArtifactMalformed, doc.States.Content[i].Value, err)
		}
		m.States = append(m.States, statemachine.State{Name: doc.States.Content[i].Value, Description: s.Description, Final: s.Final})
	}
	for _, t := range doc.Transitions {
		m.Transitions = append(m.Transitions, statemachine.Transition{
			From: t.From, To: t.To, DocumentTypes: t.DocumentTypes, Trigger: t.Trigger.toDomain(), Emits: t.Emits, Requires: t.Requires,
		})
	}
	for _, name := range slices.Sorted(maps.Keys(doc.Flags)) {
		f := doc.Flags[name]
		m.Flags = append(m.Flags, statemachine.Flag{Name: name, Description: f.Description, SetBy: f.SetBy.toDomain()})
	}
	return m, nil
}
