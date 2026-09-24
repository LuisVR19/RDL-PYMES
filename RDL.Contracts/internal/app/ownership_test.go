package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"testing"
	"testing/fstest"

	"bitbucket.org/rdl/contracts/internal/domain/ownership"
)

type fakeOwnership struct {
	m   ownership.Matrix
	err error
}

func (f fakeOwnership) Load(fs.FS, string) (ownership.Matrix, error) { return f.m, f.err }

func TestOwnershipCheckTurnsArtifactProblemsIntoFindings(t *testing.T) {
	cases := map[string]struct {
		err  error
		rule string
	}{
		"falta":       {ErrArtifactMissing, "ownership-missing"},
		"mal formado": {fmt.Errorf("%w: línea 3", ErrArtifactMalformed), "ownership-malformed"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			fs, err := NewOwnershipCheck(fakeOwnership{err: c.err}).Run(context.Background(), fstest.MapFS{})
			if err != nil || len(fs) != 1 || fs[0].Rule != c.rule || fs[0].File != OwnershipFile {
				t.Fatalf("hallazgos = %+v, err = %v", fs, err)
			}
		})
	}
}

func TestOwnershipCheckReportsViolationsWithPointer(t *testing.T) {
	m := ownership.Matrix{Schemas: []ownership.Schema{{Name: "billing", Owner: "billing",
		Access: []ownership.Access{{Service: "fiscal", Write: []ownership.Op{ownership.Insert}}}}}}
	fs, err := NewOwnershipCheck(fakeOwnership{m: m}).Run(context.Background(), fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fs {
		if f.Rule == "ownership-foreign-write" && f.Pointer == "/schemas/billing/access/fiscal/write" {
			return
		}
	}
	t.Fatalf("no se reportó la escritura ajena: %+v", fs)
}

func TestOwnershipCheckPropagatesInfrastructureErrors(t *testing.T) {
	boom := errors.New("disco")
	if _, err := NewOwnershipCheck(fakeOwnership{err: boom}).Run(context.Background(), fstest.MapFS{}); !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
}
