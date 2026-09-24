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

	"bitbucket.org/rdl/contracts/internal/domain/compat"
	"bitbucket.org/rdl/contracts/internal/domain/finding"
	"bitbucket.org/rdl/contracts/internal/domain/jsondoc"
	"bitbucket.org/rdl/contracts/internal/domain/openapi"
	"bitbucket.org/rdl/contracts/internal/domain/schema"
)

// ErrInvalidBase: el directorio base no es una copia del repo de contratos.
var ErrInvalidBase = errors.New("la base no es un repo de contratos (falta schemas/)")

// Breaking compara la versión actual con una versión base publicada (un directorio con el mismo layout, por ejemplo
// el tag anterior descomprimido) y reporta los cambios incompatibles de schemas, OpenAPI y catálogo de eventos.
type Breaking struct{ docs DocumentSource }

func NewBreaking(docs DocumentSource) *Breaking { return &Breaking{docs: docs} }

func (uc *Breaking) Execute(ctx context.Context, base, cur fs.FS) (Result, error) {
	if _, err := fs.Stat(base, schema.Dir); err != nil {
		return Result{}, ErrInvalidBase
	}
	var out []finding.Finding
	for _, step := range []func(context.Context, fs.FS, fs.FS) ([]finding.Finding, error){uc.schemas, uc.openAPI, uc.catalog} {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		fs, err := step(ctx, base, cur)
		if err != nil {
			return Result{}, err
		}
		out = append(out, fs...)
	}
	finding.Sort(out)
	return Result{Findings: out}, nil
}

func (uc *Breaking) schemas(_ context.Context, base, cur fs.FS) ([]finding.Finding, error) {
	files, err := listFiles(base, schema.Dir, schema.IsSchemaFile)
	if err != nil {
		return nil, err
	}
	var out []finding.Finding
	for _, file := range files {
		old, err := uc.docs.Load(base, file)
		if err != nil {
			return nil, fmt.Errorf("base %s: %w", file, err)
		}
		now, err := uc.docs.Load(cur, file)
		if errors.Is(err, ErrArtifactMissing) {
			out = append(out, finding.Errorf("schema-file-removed", file, "",
				"se quitó un schema publicado: una versión nueva convive con la anterior, no la reemplaza"))
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, c := range compat.Schemas(old, now) {
			out = append(out, finding.Errorf(c.Rule, file, c.Pointer, c.Message+hint(file)))
		}
	}
	return out, nil
}

// hint sugiere la salida: los eventos se versionan en un archivo nuevo.
func hint(file string) string {
	if strings.HasPrefix(file, "schemas/events/") && !strings.Contains(file, "/parts/") {
		return " (cree la versión siguiente del evento en lugar de modificar esta)"
	}
	return ""
}

func (uc *Breaking) openAPI(_ context.Context, base, cur fs.FS) ([]finding.Finding, error) {
	files, err := listFiles(base, OpenAPIDir, func(p string) bool { return strings.HasSuffix(p, ".yaml") })
	if err != nil {
		return nil, err
	}
	var out []finding.Finding
	for _, file := range files {
		oldOps, err := uc.operations(base, file)
		if err != nil {
			return nil, fmt.Errorf("base %s: %w", file, err)
		}
		newOps, err := uc.operations(cur, file)
		if errors.Is(err, ErrArtifactMissing) {
			if len(oldOps) > 0 {
				out = append(out, finding.Errorf("openapi-file-removed", file, "", "se quitó un documento OpenAPI publicado"))
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, c := range compat.Operations(oldOps, newOps) {
			out = append(out, finding.Errorf(c.Rule, file, c.Pointer, c.Message))
		}
	}
	return out, nil
}

// operations extrae las operaciones con sus parámetros resueltos (también los $ref a components/common.yaml).
func (uc *Breaking) operations(repo fs.FS, file string) ([]compat.Operation, error) {
	doc, err := uc.docs.Load(repo, file)
	if err != nil {
		return nil, err
	}
	resolve := func(v any) map[string]any {
		m := asMap(v)
		ref, ok := m["$ref"].(string)
		if !ok {
			return m
		}
		target, ptr := jsondoc.Split(ref)
		from := doc
		if target != "" {
			d, err := uc.docs.Load(repo, path.Join(path.Dir(file), target))
			if err != nil {
				return nil
			}
			from = d
		}
		r, _ := jsondoc.Resolve(from, ptr)
		return asMap(r)
	}
	paths := asMap(asMap(doc)["paths"])
	var out []compat.Operation
	for _, o := range openapi.Operations(doc) {
		item := asMap(paths[o.Path])
		op := asMap(item[strings.ToLower(o.Method)])
		var params []compat.Param
		for _, p := range append(slices.Clone(asSlice(item["parameters"])), asSlice(op["parameters"])...) {
			r := resolve(p)
			name, _ := r["name"].(string)
			in, _ := r["in"].(string)
			req, _ := r["required"].(bool)
			params = append(params, compat.Param{Name: name, In: in, Required: req})
		}
		bodyReq, _ := resolve(op["requestBody"])["required"].(bool)
		out = append(out, compat.Operation{Key: o.Key(), Pointer: o.Pointer, Params: params, BodyRequire: bodyReq})
	}
	return out, nil
}

// catalog: un evento publicado no desaparece del catálogo.
func (uc *Breaking) catalog(_ context.Context, base, cur fs.FS) ([]finding.Finding, error) {
	old, err := uc.docs.Load(base, AsyncAPIFile)
	if errors.Is(err, ErrArtifactMissing) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("base %s: %w", AsyncAPIFile, err)
	}
	now, err := uc.docs.Load(cur, AsyncAPIFile)
	if errors.Is(err, ErrArtifactMissing) {
		return []finding.Finding{finding.Errorf("catalog-removed", AsyncAPIFile, "", "se quitó el catálogo de eventos")}, nil
	}
	if err != nil {
		return nil, err
	}
	messages := func(d any) map[string]any { return asMap(asMap(asMap(d)["components"])["messages"]) }
	var out []finding.Finding
	for _, name := range slices.Sorted(maps.Keys(messages(old))) {
		if _, ok := messages(now)[name]; !ok {
			out = append(out, finding.Errorf("event-removed", AsyncAPIFile, "/components/messages/"+jsondoc.Escape(name),
				"se quitó el evento "+name+": los consumidores dependen de él"))
		}
	}
	return out, nil
}
