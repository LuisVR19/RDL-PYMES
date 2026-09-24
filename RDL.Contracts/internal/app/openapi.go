package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"bitbucket.org/rdl/contracts/internal/domain/finding"
	"bitbucket.org/rdl/contracts/internal/domain/jsondoc"
	"bitbucket.org/rdl/contracts/internal/domain/openapi"
	"bitbucket.org/rdl/contracts/internal/domain/statemachine"
)

const OpenAPIDir = "openapi"

// OpenAPIByOwner: el documento donde tiene que estar cada comando de las máquinas de estado.
var OpenAPIByOwner = map[string]string{
	"platform":    "openapi/platform.yaml",
	"billing":     "openapi/billing.yaml",
	"fiscal":      "openapi/fiscal.yaml",
	"receivables": "openapi/receivables.yaml",
}

// ImportedOpenAPI son documentos que se copian tal cual de la API que los implementa. Sus hallazgos se informan como
// avisos para esa API en lugar de corregirse aquí (el repo de la API es la fuente, CLAUDE.md).
var ImportedOpenAPI = map[string]string{
	"openapi/platform.yaml": "RDL.Platform.API/api/openapi.yaml",
}

// DocumentSource lee un YAML/JSON como árbol genérico.
type DocumentSource interface {
	Load(repo fs.FS, file string) (any, error)
}

// OpenAPICheck: cada documento cumple el meta-schema de OpenAPI 3.1, todos sus $ref resuelven (también hacia otros
// archivos del repo), cada operación tiene operationId único y los comandos de las máquinas de estado existen.
type OpenAPICheck struct {
	docs     DocumentSource
	spec     SpecValidator
	machines StateMachineSource
}

func NewOpenAPICheck(docs DocumentSource, spec SpecValidator, machines StateMachineSource) *OpenAPICheck {
	return &OpenAPICheck{docs: docs, spec: spec, machines: machines}
}

func (c *OpenAPICheck) Name() string { return "openapi" }

func (c *OpenAPICheck) Run(_ context.Context, repo fs.FS) ([]finding.Finding, error) {
	files, err := listFiles(repo, OpenAPIDir, func(p string) bool { return strings.HasSuffix(p, ".yaml") })
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return []finding.Finding{finding.Errorf("openapi-missing", OpenAPIDir, "", "no hay documentos OpenAPI")}, nil
	}

	cache := map[string]any{}
	load := func(file string) (any, error) {
		if d, ok := cache[file]; ok {
			return d, nil
		}
		d, err := c.docs.Load(repo, file)
		if err == nil {
			cache[file] = d
		}
		return d, err
	}

	var out []finding.Finding
	ops := map[string]map[string]bool{} // archivo → "MÉTODO /ruta"
	for _, file := range files {
		fs, err := c.spec.Validate(repo, file)
		if err != nil {
			return nil, err
		}
		out = append(out, fs...)

		doc, err := load(file)
		if errors.Is(err, ErrArtifactMalformed) {
			continue // ya lo reportó el meta-schema como YAML inválido
		}
		if err != nil {
			return nil, err
		}
		out = append(out, c.brokenRefs(file, doc, load)...)

		ops[file] = map[string]bool{}
		operations := openapi.Operations(doc)
		for _, o := range operations {
			ops[file][o.Key()] = true
		}
		for _, v := range openapi.OperationViolations(operations) {
			out = append(out, finding.Errorf(v.Rule, file, v.Pointer, v.Message))
		}
	}

	machines, err := c.commandsWithoutOperation(repo, ops)
	if err != nil {
		return nil, err
	}
	return downgradeImported(append(out, machines...)), nil
}

func downgradeImported(fs []finding.Finding) []finding.Finding {
	for i, f := range fs {
		if src, ok := ImportedOpenAPI[f.File]; ok && f.Severity == finding.Error {
			fs[i].Severity = finding.Warning
			fs[i].Message = "hallazgo para " + src + ": " + f.Message
		}
	}
	return fs
}

// brokenRefs verifica que cada $ref resuelva. Los $ref a JSON Schema del repo (schemas/) también se siguen.
func (c *OpenAPICheck) brokenRefs(file string, doc any, load func(string) (any, error)) []finding.Finding {
	var out []finding.Finding
	for _, r := range jsondoc.Refs(doc) {
		target, ptr := jsondoc.Split(r.Target)
		targetDoc := doc
		where := file
		if target != "" {
			if strings.Contains(target, "://") {
				out = append(out, finding.Errorf("openapi-ref", file, r.Pointer, "$ref externo "+r.Target+": solo se admiten archivos del repo"))
				continue
			}
			where = path.Join(path.Dir(file), target)
			d, err := load(where)
			if err != nil {
				out = append(out, finding.Errorf("openapi-ref", file, r.Pointer, fmt.Sprintf("$ref %s: no se puede leer %s", r.Target, where)))
				continue
			}
			targetDoc = d
		}
		if _, ok := jsondoc.Resolve(targetDoc, ptr); !ok {
			out = append(out, finding.Errorf("openapi-ref", file, r.Pointer, fmt.Sprintf("$ref %s: %s no existe en %s", r.Target, ptr, where)))
		}
	}
	return out
}

// commandsWithoutOperation: cada comando de una máquina de estado existe como operación en el OpenAPI de su dueño.
func (c *OpenAPICheck) commandsWithoutOperation(repo fs.FS, ops map[string]map[string]bool) ([]finding.Finding, error) {
	files, err := listFiles(repo, StateMachinesDir, func(p string) bool { return strings.HasSuffix(p, ".yaml") })
	if err != nil {
		return nil, err
	}
	var out []finding.Finding
	for _, file := range files {
		m, err := c.machines.Load(repo, file)
		if errors.Is(err, ErrArtifactMalformed) {
			continue // lo reporta StateMachinesCheck
		}
		if err != nil {
			return nil, err
		}
		spec, known := OpenAPIByOwner[m.Owner]
		check := func(ptr string, t statemachine.Trigger) {
			if t.Kind != statemachine.Command || !known {
				return
			}
			if !ops[spec][t.Name] {
				out = append(out, finding.Errorf("sm-command-not-in-openapi", file, ptr,
					fmt.Sprintf("%s no existe en %s", t.Name, spec)))
			}
		}
		check("/created/trigger", m.Created.Trigger)
		for i, t := range m.Transitions {
			check(fmt.Sprintf("/transitions/%d/trigger", i), t.Trigger)
		}
	}
	return out, nil
}
