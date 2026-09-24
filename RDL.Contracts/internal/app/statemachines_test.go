package app

import (
	"context"
	"fmt"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"bitbucket.org/rdl/contracts/internal/domain/statemachine"
)

type fakeMachines map[string]statemachine.Machine

func (f fakeMachines) Load(_ fs.FS, file string) (statemachine.Machine, error) {
	m, ok := f[file]
	if !ok {
		return statemachine.Machine{}, fmt.Errorf("%w: roto", ErrArtifactMalformed)
	}
	return m, nil
}

func payment() statemachine.Machine {
	return statemachine.Machine{
		Entity: "payment", Owner: "receivables",
		Created: statemachine.Creation{State: "posted", Trigger: statemachine.Trigger{Kind: statemachine.Command, Name: "POST /v1/payments"}},
		States: []statemachine.State{
			{Name: "posted", Description: "p"},
			{Name: "voided", Description: "v", Final: true},
		},
		Transitions: []statemachine.Transition{
			{From: "posted", To: "voided", Trigger: statemachine.Trigger{Kind: statemachine.Command, Name: "POST /v1/payments/{id}/void"}},
		},
	}
}

func TestStateMachinesCheck(t *testing.T) {
	m := payment()
	stale := payment()
	stale.States[1].Final = false // el doc quedó con el diagrama viejo
	wrongEntity := payment()
	wrongEntity.Entity = "pago"
	src := fakeMachines{
		"state-machines/payment.yaml": m,
		"state-machines/stale.yaml":   payment(),
		"state-machines/other.yaml":   wrongEntity,
	}
	repo := fstest.MapFS{
		"state-machines/payment.yaml":        {Data: []byte("x")},
		"state-machines/stale.yaml":          {Data: []byte("x")},
		"state-machines/other.yaml":          {Data: []byte("x")},
		"state-machines/broken.yaml":         {Data: []byte("x")},
		"docs/maquinas-de-estado/payment.md": {Data: []byte("# Pago\n\n```mermaid\n" + m.Mermaid() + "```\n")},
		"docs/maquinas-de-estado/stale.md":   {Data: []byte("```mermaid\n" + stale.Mermaid() + "```\n")},
	}
	got, err := NewStateMachinesCheck(src).Run(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, f := range got {
		keys = append(keys, f.Rule+" "+f.File)
	}
	for _, want := range []string{
		"sm-doc-drift docs/maquinas-de-estado/stale.md",
		"sm-doc-missing docs/maquinas-de-estado/other.md",
		"sm-entity state-machines/other.yaml",
		"sm-malformed state-machines/broken.yaml",
	} {
		if !slices.Contains(keys, want) {
			t.Errorf("falta %q en %v", want, keys)
		}
	}
	for _, k := range keys {
		if k == "sm-doc-drift docs/maquinas-de-estado/payment.md" {
			t.Error("payment.md está al día y se reportó como desincronizado")
		}
	}
}

func TestDiagram(t *testing.T) {
	out, err := Diagram(fakeMachines{"sm.yaml": payment()}, fstest.MapFS{}, "sm.yaml")
	if err != nil || out != "```mermaid\n"+payment().Mermaid()+"```\n" {
		t.Fatalf("out = %q, err = %v", out, err)
	}
}
