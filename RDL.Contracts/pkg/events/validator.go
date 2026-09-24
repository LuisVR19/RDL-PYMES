package events

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	js "github.com/santhosh-tekuri/jsonschema/v6"

	contracts "bitbucket.org/rdl/contracts"
)

const schemaBaseURI = "https://contracts.rdl.invalid/"

// Validator valida payloads contra los schemas embebidos en este módulo (la misma versión que los DTOs).
// Es seguro para uso concurrente; compile una vez al arrancar y reutilícelo.
type Validator struct {
	schemas map[string]*js.Schema
}

var (
	defaultOnce      sync.Once
	defaultValidator *Validator
	defaultErr       error
)

// DefaultValidator compila los schemas una sola vez por proceso.
func DefaultValidator() (*Validator, error) {
	defaultOnce.Do(func() { defaultValidator, defaultErr = NewValidator() })
	return defaultValidator, defaultErr
}

func NewValidator() (*Validator, error) {
	c := js.NewCompiler()
	c.DefaultDraft(js.Draft2020)
	c.AssertFormat()
	c.UseLoader(noNetwork{})

	var files []string
	err := fs.WalkDir(contracts.Schemas, "schemas", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".json") {
			return err
		}
		raw, err := fs.ReadFile(contracts.Schemas, p)
		if err != nil {
			return err
		}
		doc, err := js.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		files = append(files, p)
		return c.AddResource(schemaBaseURI+p, doc)
	})
	if err != nil {
		return nil, fmt.Errorf("cargando schemas: %w", err)
	}
	v := &Validator{schemas: make(map[string]*js.Schema, len(files))}
	for _, f := range files {
		sch, err := c.Compile(schemaBaseURI + f)
		if err != nil {
			return nil, fmt.Errorf("compilando %s: %w", f, err)
		}
		v.schemas[f] = sch
	}
	return v, nil
}

// Validate valida el JSON serializado contra el schema (por ejemplo InvoiceIssuedSchemaV1). Devuelve un error que
// envuelve ErrInvalid con la ubicación de cada problema.
func (v *Validator) Validate(schemaFile string, payload []byte) error {
	sch, ok := v.schemas[schemaFile]
	if !ok {
		return fmt.Errorf("schema %q desconocido", schemaFile)
	}
	inst, err := js.UnmarshalJSON(bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("%w: JSON mal formado: %w", ErrInvalid, err)
	}
	err = sch.Validate(inst)
	if err == nil {
		return nil
	}
	var ve *js.ValidationError
	if errors.As(err, &ve) {
		var msgs []string
		for _, u := range ve.BasicOutput().Errors {
			if u.Error != nil && len(u.Errors) == 0 {
				msgs = append(msgs, u.InstanceLocation+": "+u.Error.String())
			}
		}
		if len(msgs) > 0 {
			return fmt.Errorf("%w: %s", ErrInvalid, strings.Join(msgs, "; "))
		}
	}
	return fmt.Errorf("%w: %w", ErrInvalid, err)
}

type noNetwork struct{}

func (noNetwork) Load(url string) (any, error) {
	return nil, fmt.Errorf("$ref a %s: solo se admiten schemas embebidos", url)
}
