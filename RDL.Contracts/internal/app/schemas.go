package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"bitbucket.org/rdl/contracts/internal/domain/finding"
	"bitbucket.org/rdl/contracts/internal/domain/jsonpatch"
	"bitbucket.org/rdl/contracts/internal/domain/schema"
)

// SchemaSet valida instancias contra los schemas compilados. Validate devuelve un mensaje por problema; vacío = válido.
type SchemaSet interface {
	Validate(file string, instance any) []string
}

// SchemaCompiler compila los schemas del repo. Los problemas de un archivo (JSON inválido, $id incorrecto, $ref que
// no resuelve) son hallazgos; error solo si falla la lectura.
type SchemaCompiler interface {
	Compile(repo fs.FS, files []string) (SchemaSet, []finding.Finding, error)
}

// SchemasCheck: todos los JSON Schema del repo compilan con el dialecto y el $id acordados.
type SchemasCheck struct{ compiler SchemaCompiler }

func NewSchemasCheck(c SchemaCompiler) *SchemasCheck { return &SchemasCheck{compiler: c} }

func (c *SchemasCheck) Name() string { return "schemas" }

func (c *SchemasCheck) Run(_ context.Context, repo fs.FS) ([]finding.Finding, error) {
	files, err := listFiles(repo, schema.Dir, schema.IsSchemaFile)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return []finding.Finding{finding.Errorf("schemas-missing", schema.Dir, "", "no hay schemas")}, nil
	}
	_, fs, err := c.compiler.Compile(repo, files)
	return fs, err
}

// ExamplesDir guarda suites de ejemplos:
// {"description": "...", "schema": "<ruta>", "valid": [...], "invalid": [{"value": ..., "why": "..."}]}.
const ExamplesDir = "examples"

type exampleSuite struct {
	Description string            `json:"description"`
	Schema      string            `json:"schema"`
	Valid       []json.RawMessage `json:"valid"`
	Invalid     []invalidExample  `json:"invalid"`
}

// invalidExample es un valor completo (value) o un válido con un parche (base + patch, RFC 6902).
type invalidExample struct {
	Value json.RawMessage `json:"value"`
	Base  *int            `json:"base"`
	Patch []jsonpatch.Op  `json:"patch"`
	Why   string          `json:"why"`
}

// instance construye el valor inválido. err describe un ejemplo mal escrito.
func (inv invalidExample) instance(valid []json.RawMessage) (any, error) {
	switch {
	case inv.Base == nil && len(inv.Value) > 0 && len(inv.Patch) == 0:
		return decode(inv.Value), nil
	case inv.Base != nil && len(inv.Value) == 0 && len(inv.Patch) > 0:
		if *inv.Base < 0 || *inv.Base >= len(valid) {
			return nil, fmt.Errorf("base %d no es un índice de valid", *inv.Base)
		}
		return jsonpatch.Apply(decode(valid[*inv.Base]), inv.Patch)
	}
	return nil, errors.New("use value, o base + patch")
}

// ExamplesCheck: cada ejemplo válido pasa su schema y cada inválido lo rechaza. Los inválidos documentan qué se
// debe rechazar (monto como number, fecha sin Z, campo extra...): si uno pasa, el schema es demasiado permisivo.
type ExamplesCheck struct{ compiler SchemaCompiler }

func NewExamplesCheck(c SchemaCompiler) *ExamplesCheck { return &ExamplesCheck{compiler: c} }

func (c *ExamplesCheck) Name() string { return "examples" }

func (c *ExamplesCheck) Run(_ context.Context, repo fs.FS) ([]finding.Finding, error) {
	schemaFiles, err := listFiles(repo, schema.Dir, schema.IsSchemaFile)
	if err != nil {
		return nil, err
	}
	set, compileProblems, err := c.compiler.Compile(repo, schemaFiles)
	if err != nil {
		return nil, err
	}
	if len(compileProblems) > 0 {
		return nil, nil // los reporta SchemasCheck; validar ejemplos contra schemas rotos solo agrega ruido
	}
	suites, err := listFiles(repo, ExamplesDir, func(p string) bool { return strings.HasSuffix(p, ".json") })
	if err != nil {
		return nil, err
	}

	var out []finding.Finding
	for _, file := range suites {
		raw, err := fs.ReadFile(repo, file)
		if err != nil {
			return nil, fmt.Errorf("leyendo %s: %w", file, err)
		}
		var suite exampleSuite
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		dec.UseNumber() // los value de los parches conservan el tipo exacto (un number sigue siendo number)
		if err := dec.Decode(&suite); err != nil {
			out = append(out, finding.Errorf("examples-malformed", file, "", err.Error()))
			continue
		}
		if !schema.IsSchemaFile(suite.Schema) {
			out = append(out, finding.Errorf("examples-unknown-schema", file, "/schema", fmt.Sprintf("%q no es un schema del repo", suite.Schema)))
			continue
		}
		if len(suite.Valid) == 0 || len(suite.Invalid) == 0 {
			out = append(out, finding.Errorf("examples-incomplete", file, "", "se necesita al menos un ejemplo válido y uno inválido"))
		}
		for i, v := range suite.Valid {
			for _, msg := range set.Validate(suite.Schema, decode(v)) {
				out = append(out, finding.Errorf("example-valid-rejected", file, fmt.Sprintf("/valid/%d", i), msg))
			}
		}
		for i, inv := range suite.Invalid {
			if inv.Why == "" {
				out = append(out, finding.Errorf("example-invalid-without-reason", file, fmt.Sprintf("/invalid/%d/why", i), "explique por qué debe rechazarse"))
			}
			v, err := inv.instance(suite.Valid)
			if err != nil {
				out = append(out, finding.Errorf("example-invalid-malformed", file, fmt.Sprintf("/invalid/%d", i), err.Error()))
				continue
			}
			if len(set.Validate(suite.Schema, v)) == 0 {
				out = append(out, finding.Errorf("example-invalid-accepted", file, fmt.Sprintf("/invalid/%d", i),
					fmt.Sprintf("el schema acepta un valor que debería rechazar (%s)", inv.Why)))
			}
		}
	}
	return out, nil
}

// decode convierte un ejemplo a los tipos que espera el validador, conservando los números como json.Number para no
// perder precisión (justo lo que se quiere detectar en los montos).
func decode(raw json.RawMessage) any {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil
	}
	return v
}

func listFiles(repo fs.FS, dir string, keep func(string) bool) ([]string, error) {
	var files []string
	err := fs.WalkDir(repo, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && keep(path.Clean(p)) {
			files = append(files, path.Clean(p))
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return files, err
}
