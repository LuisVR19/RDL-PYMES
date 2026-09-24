package app

import (
	"context"
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"

	"bitbucket.org/rdl/contracts/internal/domain/finding"
)

type fakeCheck struct {
	name     string
	findings []finding.Finding
	err      error
}

func (c fakeCheck) Name() string { return c.name }
func (c fakeCheck) Run(context.Context, fs.FS) ([]finding.Finding, error) {
	return c.findings, c.err
}

func TestRunChecksJoinsAndSortsFindings(t *testing.T) {
	uc := NewRunChecks(
		fakeCheck{name: "b", findings: []finding.Finding{finding.Errorf("r", "z.json", "", "m")}},
		fakeCheck{name: "a", findings: []finding.Finding{finding.Warnf("r", "a.json", "", "m")}},
	)
	res, err := uc.Execute(context.Background(), fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Findings) != 2 || res.Findings[0].File != "a.json" {
		t.Fatalf("hallazgos = %+v", res.Findings)
	}
	if !res.Failed() {
		t.Fatal("con un error el resultado debe fallar")
	}
}

func TestRunChecksWithoutFindingsPasses(t *testing.T) {
	res, err := NewRunChecks().Execute(context.Background(), fstest.MapFS{})
	if err != nil || res.Failed() {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestRunChecksStopsOnInfrastructureError(t *testing.T) {
	boom := errors.New("no se pudo leer")
	_, err := NewRunChecks(fakeCheck{name: "ownership", err: boom}).Execute(context.Background(), fstest.MapFS{})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}
