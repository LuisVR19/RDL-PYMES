package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"

	"bitbucket.org/rdl/contracts/internal/domain/catalog"
	"bitbucket.org/rdl/contracts/internal/domain/finding"
)

const AsyncAPIFile = "asyncapi/asyncapi.yaml"

type CatalogSource interface {
	Load(repo fs.FS, file string) ([]catalog.Event, error)
}

// SpecValidator valida un documento contra el meta-schema de su especificación (AsyncAPI, OpenAPI).
type SpecValidator interface {
	Validate(repo fs.FS, file string) ([]finding.Finding, error)
}

// CatalogCheck: el documento AsyncAPI es válido y su catálogo coincide con la arquitectura (6.2) y con los schemas.
type CatalogCheck struct {
	src  CatalogSource
	spec SpecValidator
}

func NewCatalogCheck(src CatalogSource, spec SpecValidator) *CatalogCheck {
	return &CatalogCheck{src: src, spec: spec}
}

func (c *CatalogCheck) Name() string { return "catalog" }

func (c *CatalogCheck) Run(_ context.Context, repo fs.FS) ([]finding.Finding, error) {
	out, err := c.spec.Validate(repo, AsyncAPIFile)
	if errors.Is(err, ErrArtifactMissing) {
		return []finding.Finding{finding.Errorf("asyncapi-missing", AsyncAPIFile, "", "no existe el catálogo de eventos")}, nil
	}
	if err != nil {
		return nil, err
	}
	events, err := c.src.Load(repo, AsyncAPIFile)
	switch {
	case errors.Is(err, ErrArtifactMalformed):
		return append(out, finding.Errorf("asyncapi-malformed", AsyncAPIFile, "", err.Error())), nil
	case err != nil:
		return nil, err
	}

	consts := map[string]catalog.SchemaConsts{}
	for _, ev := range events {
		if ev.SchemaFile == "" {
			continue
		}
		sc, err := readConsts(repo, ev.SchemaFile)
		if err != nil {
			return nil, err
		}
		consts[ev.SchemaFile] = sc
	}
	for _, v := range catalog.Check(events, catalog.Architecture62, consts) {
		out = append(out, finding.Errorf(v.Rule, AsyncAPIFile, v.Pointer, v.Message))
	}
	return out, nil
}

// readConsts lee los const de eventType, version y sourceService del schema de un evento.
func readConsts(repo fs.FS, file string) (catalog.SchemaConsts, error) {
	raw, err := fs.ReadFile(repo, file)
	if errors.Is(err, fs.ErrNotExist) {
		return catalog.SchemaConsts{}, nil
	}
	if err != nil {
		return catalog.SchemaConsts{}, fmt.Errorf("leyendo %s: %w", file, err)
	}
	var doc struct {
		Properties struct {
			EventType     struct{ Const string } `json:"eventType"`
			Version       struct{ Const int }    `json:"version"`
			SourceService struct{ Const string } `json:"sourceService"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		// El JSON inválido lo reporta SchemasCheck; aquí el schema cuenta como presente pero sin const.
		return catalog.SchemaConsts{Found: true}, nil
	}
	p := doc.Properties
	return catalog.SchemaConsts{Found: true, EventType: p.EventType.Const, Version: p.Version.Const, SourceService: p.SourceService.Const}, nil
}
