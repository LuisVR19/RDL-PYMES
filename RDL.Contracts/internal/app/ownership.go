package app

import (
	"context"
	"errors"
	"io/fs"

	"bitbucket.org/rdl/contracts/internal/domain/finding"
	"bitbucket.org/rdl/contracts/internal/domain/ownership"
)

const OwnershipFile = "ownership/ownership.yaml"

// Errores que los adapters de lectura devuelven envueltos: el caso de uso los convierte en hallazgos (el archivo falta
// o está mal escrito es un problema del contrato, no de infraestructura).
var (
	ErrArtifactMissing   = errors.New("artefacto inexistente")
	ErrArtifactMalformed = errors.New("artefacto mal formado")
)

type OwnershipSource interface {
	Load(repo fs.FS, path string) (ownership.Matrix, error)
}

// OwnershipCheck valida la matriz de ownership (arquitectura 2.1).
type OwnershipCheck struct{ src OwnershipSource }

func NewOwnershipCheck(src OwnershipSource) *OwnershipCheck { return &OwnershipCheck{src: src} }

func (c *OwnershipCheck) Name() string { return "ownership" }

func (c *OwnershipCheck) Run(_ context.Context, repo fs.FS) ([]finding.Finding, error) {
	m, err := c.src.Load(repo, OwnershipFile)
	switch {
	case errors.Is(err, ErrArtifactMissing):
		return []finding.Finding{finding.Errorf("ownership-missing", OwnershipFile, "", "no existe la matriz de ownership")}, nil
	case errors.Is(err, ErrArtifactMalformed):
		return []finding.Finding{finding.Errorf("ownership-malformed", OwnershipFile, "", err.Error())}, nil
	case err != nil:
		return nil, err
	}
	var out []finding.Finding
	for _, v := range m.Violations() {
		out = append(out, finding.Errorf(v.Rule, OwnershipFile, v.Pointer, v.Message))
	}
	return out, nil
}
