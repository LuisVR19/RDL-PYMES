package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"bitbucket.org/rdl/contracts/internal/domain/catalog"
	"bitbucket.org/rdl/contracts/internal/domain/finding"
	"bitbucket.org/rdl/contracts/internal/domain/statemachine"
)

const (
	StateMachinesDir     = "state-machines"
	StateMachinesDocsDir = "docs/maquinas-de-estado"
)

type StateMachineSource interface {
	Load(repo fs.FS, file string) (statemachine.Machine, error)
}

// StateMachinesCheck: cada máquina cumple sus invariantes y su doc incluye el diagrama que sale del YAML.
type StateMachinesCheck struct{ src StateMachineSource }

func NewStateMachinesCheck(src StateMachineSource) *StateMachinesCheck {
	return &StateMachinesCheck{src: src}
}

func (c *StateMachinesCheck) Name() string { return "state-machines" }

func (c *StateMachinesCheck) Run(_ context.Context, repo fs.FS) ([]finding.Finding, error) {
	files, err := listFiles(repo, StateMachinesDir, func(p string) bool { return strings.HasSuffix(p, ".yaml") })
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return []finding.Finding{finding.Errorf("sm-missing", StateMachinesDir, "", "no hay máquinas de estado")}, nil
	}
	var out []finding.Finding
	for _, file := range files {
		m, err := c.src.Load(repo, file)
		switch {
		case errors.Is(err, ErrArtifactMalformed):
			out = append(out, finding.Errorf("sm-malformed", file, "", err.Error()))
			continue
		case err != nil:
			return nil, err
		}
		if want := strings.TrimSuffix(path.Base(file), ".yaml"); m.Entity != want {
			out = append(out, finding.Errorf("sm-entity", file, "/entity", fmt.Sprintf("entity debe ser %q (el nombre del archivo)", want)))
		}
		for _, v := range m.Violations(catalog.Architecture62) {
			out = append(out, finding.Errorf(v.Rule, file, v.Pointer, v.Message))
		}
		out = append(out, c.docDrift(repo, file, m)...)
	}
	return out, nil
}

// docDrift exige que el doc de la máquina contenga el diagrama exacto que sale del YAML (contractsctl diagram).
func (c *StateMachinesCheck) docDrift(repo fs.FS, file string, m statemachine.Machine) []finding.Finding {
	doc := path.Join(StateMachinesDocsDir, strings.TrimSuffix(path.Base(file), ".yaml")+".md")
	raw, err := fs.ReadFile(repo, doc)
	if err != nil {
		return []finding.Finding{finding.Errorf("sm-doc-missing", doc, "", "falta el documento de la máquina "+file)}
	}
	if !strings.Contains(string(raw), "```mermaid\n"+m.Mermaid()+"```") {
		return []finding.Finding{finding.Errorf("sm-doc-drift", doc, "",
			"el diagrama no coincide con "+file+": regenérelo con `contractsctl diagram "+file+"`")}
	}
	return nil
}

// Diagram devuelve el bloque Mermaid de una máquina (para pegarlo en su doc).
func Diagram(src StateMachineSource, repo fs.FS, file string) (string, error) {
	m, err := src.Load(repo, file)
	if err != nil {
		return "", err
	}
	return "```mermaid\n" + m.Mermaid() + "```\n", nil
}
