package wiring

import (
	"context"
	"os"
	"testing"

	"bitbucket.org/rdl/contracts/internal/app"
)

// Los artefactos reales del repo tienen que pasar: si alguien rompe un contrato, falla `go test`, no solo el CI.
func TestRepoArtifactsPass(t *testing.T) {
	repo := os.DirFS("../..")
	for name, uc := range map[string]*app.RunChecks{"validate": Validate(), "lint": Lint()} {
		res, err := uc.Execute(context.Background(), repo)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, f := range res.Findings {
			t.Logf("%s: %s %s#%s [%s] %s", name, f.Severity, f.File, f.Pointer, f.Rule, f.Message)
		}
		if res.Failed() {
			t.Errorf("%s falla sobre los artefactos del repo", name)
		}
	}
}
