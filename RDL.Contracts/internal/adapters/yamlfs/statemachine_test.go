package yamlfs

import (
	"errors"
	"testing"
	"testing/fstest"

	"bitbucket.org/rdl/contracts/internal/app"
)

func TestStateMachineLoaderKeepsStateOrder(t *testing.T) {
	repo := fstest.MapFS{"sm.yaml": {Data: []byte(`
version: 1
entity: payment
owner: receivables
created:
  state: posted
  trigger: {kind: command, name: POST /v1/payments}
  emits: PaymentReceived
states:
  zeta: {description: z}
  posted: {description: p}
  voided: {description: v, final: true}
transitions:
  - from: posted
    to: voided
    trigger: {kind: command, name: "POST /v1/payments/{id}/void"}
    requires: [motivo]
flags:
  x: {description: d, setBy: {kind: event, name: InvoiceIssued}}
`)}}
	m, err := StateMachineLoader{}.Load(repo, "sm.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.States) != 3 || m.States[0].Name != "zeta" || m.States[1].Name != "posted" || !m.States[2].Final {
		t.Fatalf("estados = %+v", m.States)
	}
	if m.Created.Emits != "PaymentReceived" || m.Transitions[0].Trigger.Name != "POST /v1/payments/{id}/void" ||
		m.Transitions[0].Requires[0] != "motivo" || m.Flags[0].SetBy.Name != "InvoiceIssued" {
		t.Fatalf("máquina = %+v", m)
	}
}

func TestStateMachineLoaderRejectsUnknownKeys(t *testing.T) {
	repo := fstest.MapFS{"sm.yaml": {Data: []byte("version: 1\nentity: x\ntransitons: []\n")}}
	if _, err := (StateMachineLoader{}).Load(repo, "sm.yaml"); !errors.Is(err, app.ErrArtifactMalformed) {
		t.Fatalf("err = %v", err)
	}
	if _, err := (StateMachineLoader{}).Load(fstest.MapFS{}, "sm.yaml"); !errors.Is(err, app.ErrArtifactMissing) {
		t.Fatalf("err = %v", err)
	}
}
