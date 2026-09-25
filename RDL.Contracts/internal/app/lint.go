package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"bitbucket.org/rdl/contracts/internal/domain/convention"
	"bitbucket.org/rdl/contracts/internal/domain/finding"
	"bitbucket.org/rdl/contracts/internal/domain/jsondoc"
	"bitbucket.org/rdl/contracts/internal/domain/ownership"
	"bitbucket.org/rdl/contracts/internal/domain/schema"
)

const ProblemsDir = "problems"

// LintCheck aplica docs/convenciones.md a los JSON Schema, los OpenAPI y el registro de problem types.
type LintCheck struct{ docs DocumentSource }

func NewLintCheck(docs DocumentSource) *LintCheck { return &LintCheck{docs: docs} }

func (c *LintCheck) Name() string { return "conventions" }

func (c *LintCheck) Run(_ context.Context, repo fs.FS) ([]finding.Finding, error) {
	var out []finding.Finding
	emit := func(file string, vs []convention.Violation) {
		for _, v := range vs {
			out = append(out, finding.Errorf(v.Rule, file, v.Pointer, v.Message))
		}
	}
	load := func(file string) (any, bool, error) {
		doc, err := c.docs.Load(repo, file)
		if errors.Is(err, ErrArtifactMalformed) {
			return nil, false, nil // lo reporta validate
		}
		return doc, err == nil, err
	}

	schemas, err := listFiles(repo, schema.Dir, schema.IsSchemaFile)
	if err != nil {
		return nil, err
	}
	for _, file := range schemas {
		doc, ok, err := load(file)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		emit(file, convention.SchemaRules(doc, ""))
		emit(file, convention.ClosedObjects(doc))
		if isEventSchema(file) {
			emit(file, convention.EventRules(doc))
		}
	}

	specs, err := listFiles(repo, OpenAPIDir, func(p string) bool { return strings.HasSuffix(p, ".yaml") })
	if err != nil {
		return nil, err
	}
	for _, file := range specs {
		doc, ok, err := load(file)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		emit(file, convention.OpenAPIRules(doc))
		for _, s := range openAPISchemas(doc) {
			emit(file, convention.SchemaRules(s.node, s.ptr))
		}
	}

	problems, err := c.problems(repo)
	if err != nil {
		return nil, err
	}
	out = append(out, problems...)
	return downgradeImported(out), nil
}

// isEventSchema: schemas/events/<evento>.vN.json, sin contar el sobre ni las partes.
func isEventSchema(file string) bool {
	return path.Dir(file) == "schemas/events" && path.Base(file) != "envelope.v1.json"
}

type located struct {
	node any
	ptr  string
}

// openAPISchemas encuentra los schemas embebidos: components/schemas/* y todo valor de una clave `schema`.
func openAPISchemas(doc any) []located {
	var out []located
	for _, name := range slices.Sorted(maps.Keys(asMap(asMap(asMap(doc)["components"])["schemas"]))) {
		out = append(out, located{asMap(asMap(asMap(doc)["components"])["schemas"])[name], "/components/schemas/" + jsondoc.Escape(name)})
	}
	var walk func(v any, ptr string)
	walk = func(v any, ptr string) {
		switch t := v.(type) {
		case map[string]any:
			for _, k := range slices.Sorted(maps.Keys(t)) {
				if k == "schema" && !strings.HasPrefix(ptr, "/components/schemas") {
					out = append(out, located{t[k], ptr + "/schema"})
					continue
				}
				walk(t[k], ptr+"/"+jsondoc.Escape(k))
			}
		case []any:
			for i, x := range t {
				walk(x, fmt.Sprintf("%s/%d", ptr, i))
			}
		}
	}
	walk(asMap(doc)["paths"], "/paths")
	walk(asMap(asMap(doc)["components"])["parameters"], "/components/parameters")
	walk(asMap(asMap(doc)["components"])["responses"], "/components/responses")
	return out
}

// problems valida problems/<servicio>.yaml para los cuatro servicios de dominio y los de borde.
func (c *LintCheck) problems(repo fs.FS) ([]finding.Finding, error) {
	var out []finding.Finding
	services := append(slices.Clone(ownership.Services), slices.Sorted(maps.Keys(convention.EdgeProblems))...)
	for _, svc := range services {
		file := path.Join(ProblemsDir, svc+".yaml")
		doc, err := c.docs.Load(repo, file)
		switch {
		case errors.Is(err, ErrArtifactMissing):
			out = append(out, finding.Errorf("problem-registry", file, "", "falta el registro de problem types de "+svc))
			continue
		case errors.Is(err, ErrArtifactMalformed):
			out = append(out, finding.Errorf("problem-registry", file, "", err.Error()))
			continue
		case err != nil:
			return nil, err
		}
		root := asMap(doc)
		if root["service"] != svc {
			out = append(out, finding.Errorf("problem-registry", file, "/service", "service debe ser "+svc))
		}
		var ps []convention.Problem
		for _, p := range asSlice(root["problems"]) {
			m := asMap(p)
			code, _ := m["code"].(string)
			status, _ := m["status"].(int)
			title, _ := m["title"].(string)
			when, _ := m["when"].(string)
			ps = append(ps, convention.Problem{Code: code, Status: status, UpstreamStatus: m["status"] == "upstream",
				Title: title, When: when})
		}
		for _, v := range convention.ProblemRules(svc, ps) {
			out = append(out, finding.Errorf(v.Rule, file, v.Pointer, v.Message))
		}
	}
	return out, nil
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}
