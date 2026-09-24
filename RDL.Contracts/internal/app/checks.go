// Package app contiene los casos de uso del CLI y los puertos que necesitan.
package app

import (
	"context"
	"fmt"
	"io/fs"

	"bitbucket.org/rdl/contracts/internal/domain/finding"
)

// Check es una verificación sobre los artefactos del repo. Una regla nueva es un Check más en la lista de wiring:
// los casos de uso no cambian.
type Check interface {
	Name() string
	Run(ctx context.Context, repo fs.FS) ([]finding.Finding, error)
}

type Result struct {
	Findings []finding.Finding
}

func (r Result) Failed() bool { return finding.HasErrors(r.Findings) }

// RunChecks ejecuta una lista de verificaciones y junta sus hallazgos. `validate` y `lint` son dos instancias con
// listas distintas.
type RunChecks struct{ checks []Check }

func NewRunChecks(checks ...Check) *RunChecks { return &RunChecks{checks: checks} }

func (uc *RunChecks) Execute(ctx context.Context, repo fs.FS) (Result, error) {
	var all []finding.Finding
	for _, c := range uc.checks {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		fs, err := c.Run(ctx, repo)
		if err != nil {
			// Un error aquí es de infraestructura (no se pudo leer), no un hallazgo: se corta en lugar de seguir a ciegas.
			return Result{}, fmt.Errorf("%s: %w", c.Name(), err)
		}
		all = append(all, fs...)
	}
	finding.Sort(all)
	return Result{Findings: all}, nil
}
