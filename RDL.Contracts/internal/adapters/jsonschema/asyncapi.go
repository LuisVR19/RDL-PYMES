package jsonschema

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sync"

	js "github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"

	"bitbucket.org/rdl/contracts/internal/app"
	"bitbucket.org/rdl/contracts/internal/domain/finding"
)

// Meta-schemas oficiales vendorizados para validar sin red (ver metaschemas/README.md).
var (
	//go:embed metaschemas/asyncapi-3.0.0.json
	asyncAPIMetaschema []byte
	//go:embed metaschemas/openapi-3.1-2022-10-07.json
	openAPIMetaschema []byte
)

// SpecValidator valida un documento YAML (AsyncAPI u OpenAPI) contra el meta-schema oficial de su especificación.
type SpecValidator struct {
	rule string
	meta []byte
	id   string

	once   *sync.Once
	schema **js.Schema
	err    *error
}

func newSpecValidator(rule string, meta []byte, id string) SpecValidator {
	var s *js.Schema
	var err error
	return SpecValidator{rule: rule, meta: meta, id: id, once: &sync.Once{}, schema: &s, err: &err}
}

var (
	asyncAPI30 = newSpecValidator("asyncapi-spec", asyncAPIMetaschema, "http://asyncapi.com/definitions/3.0.0/asyncapi.json")
	openAPI31  = newSpecValidator("openapi-spec", openAPIMetaschema, "https://spec.openapis.org/oas/3.1/schema/2022-10-07")
)

// AsyncAPI30 valida contra el meta-schema de AsyncAPI 3.0.0 (draft-07, bundle autocontenido).
func AsyncAPI30() SpecValidator { return asyncAPI30 }

// OpenAPI31 valida contra el meta-schema de OpenAPI 3.1 (2020-12). No valida los Schema Objects por dentro: los tipos
// de datos son los JSON Schema del repo, que ya compila SchemasCheck.
func OpenAPI31() SpecValidator { return openAPI31 }

func (v SpecValidator) compiled() (*js.Schema, error) {
	v.once.Do(func() {
		doc, err := js.UnmarshalJSON(bytes.NewReader(v.meta))
		if err != nil {
			*v.err = err
			return
		}
		c := js.NewCompiler()
		c.UseLoader(noNetwork{})
		if err := c.AddResource(v.id, doc); err != nil {
			*v.err = err
			return
		}
		*v.schema, *v.err = c.Compile(v.id)
	})
	return *v.schema, *v.err
}

func (v SpecValidator) Validate(repo fs.FS, file string) ([]finding.Finding, error) {
	sch, err := v.compiled()
	if err != nil {
		return nil, fmt.Errorf("compilando el meta-schema %s: %w", v.id, err)
	}
	raw, err := fs.ReadFile(repo, file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, app.ErrArtifactMissing
	}
	if err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", file, err)
	}
	var doc any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return []finding.Finding{finding.Errorf(v.rule, file, "", "YAML inválido: "+err.Error())}, nil
	}
	// YAML → JSON → modelo del validador: así los números y las claves quedan como en un documento JSON.
	asJSON, err := json.Marshal(doc)
	if err != nil {
		return []finding.Finding{finding.Errorf(v.rule, file, "", err.Error())}, nil
	}
	inst, err := js.UnmarshalJSON(bytes.NewReader(asJSON))
	if err != nil {
		return nil, err
	}
	var out []finding.Finding
	for _, msg := range (set{byFile: map[string]*js.Schema{file: sch}}).Validate(file, inst) {
		out = append(out, finding.Errorf(v.rule, file, "", msg))
	}
	return out, nil
}
