// Package jsonschema compila y aplica los JSON Schema 2020-12 del repo con santhosh-tekuri/jsonschema.
package jsonschema

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	js "github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"bitbucket.org/rdl/contracts/internal/app"
	"bitbucket.org/rdl/contracts/internal/domain/finding"
	"bitbucket.org/rdl/contracts/internal/domain/schema"
)

type Compiler struct{}

type set struct{ byFile map[string]*js.Schema }

func (s set) Validate(file string, instance any) []string {
	sch, ok := s.byFile[file]
	if !ok {
		return []string{fmt.Sprintf("schema %s no compilado", file)}
	}
	err := sch.Validate(instance)
	if err == nil {
		return nil
	}
	var ve *js.ValidationError
	if !errors.As(err, &ve) {
		return []string{err.Error()}
	}
	msgs := leafMessages(ve)
	if len(msgs) == 0 {
		msgs = append(msgs, ve.Error())
	}
	return msgs
}

// leafMessages devuelve las causas hoja: "validation failed" en un $ref o un oneOf no dice qué corregir.
// Se deduplican porque un oneOf repite la misma causa por cada rama.
func leafMessages(ve *js.ValidationError) []string {
	var out []string
	seen := map[string]bool{}
	var walk func(*js.ValidationError)
	walk = func(e *js.ValidationError) {
		if len(e.Causes) > 0 {
			for _, c := range e.Causes {
				walk(c)
			}
			return
		}
		loc := "/" + strings.Join(e.InstanceLocation, "/")
		msg := loc + ": " + e.ErrorKind.LocalizedString(printer)
		if !seen[msg] {
			seen[msg] = true
			out = append(out, msg)
		}
	}
	walk(ve)
	return out
}

var printer = message.NewPrinter(language.Spanish)

func (Compiler) Compile(repo fs.FS, files []string) (app.SchemaSet, []finding.Finding, error) {
	c := js.NewCompiler()
	c.DefaultDraft(js.Draft2020)
	c.AssertFormat() // uuid, date-time, date y email se validan, no son solo anotaciones
	c.UseLoader(noNetwork{})

	var out []finding.Finding
	var added []string
	for _, file := range files {
		raw, err := fs.ReadFile(repo, file)
		if err != nil {
			return nil, nil, fmt.Errorf("leyendo %s: %w", file, err)
		}
		doc, err := js.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			out = append(out, finding.Errorf("schema-json", file, "", err.Error()))
			continue
		}
		obj, _ := doc.(map[string]any)
		if obj["$schema"] != schema.Dialect {
			out = append(out, finding.Errorf("schema-dialect", file, "/$schema", "debe ser "+schema.Dialect))
		}
		if want := schema.ExpectedID(file); obj["$id"] != want {
			out = append(out, finding.Errorf("schema-id", file, "/$id", fmt.Sprintf("debe ser %q (refleja la ruta del archivo)", want)))
		}
		if err := c.AddResource(schema.ExpectedID(file), doc); err != nil {
			out = append(out, finding.Errorf("schema-resource", file, "", err.Error()))
			continue
		}
		added = append(added, file)
	}

	s := set{byFile: map[string]*js.Schema{}}
	for _, file := range added {
		sch, err := c.Compile(schema.ExpectedID(file))
		if err != nil {
			out = append(out, finding.Errorf("schema-compile", file, "", strings.ReplaceAll(err.Error(), schema.BaseURI, "")))
			continue
		}
		s.byFile[file] = sch
	}
	return s, out, nil
}

// noNetwork impide que un $ref a una URL externa se descargue: todo schema tiene que estar en el repo.
type noNetwork struct{}

func (noNetwork) Load(url string) (any, error) {
	return nil, fmt.Errorf("$ref a %s: solo se admiten schemas del repo", url)
}
