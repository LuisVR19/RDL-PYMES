package app

import (
	"context"
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"bitbucket.org/rdl/contracts/internal/domain/catalog"
	"bitbucket.org/rdl/contracts/internal/domain/finding"
)

type fakeCatalog struct{ events []catalog.Event }

func (f fakeCatalog) Load(fs.FS, string) ([]catalog.Event, error) { return f.events, nil }

type fakeSpec struct {
	findings []finding.Finding
	err      error
}

func (f fakeSpec) Validate(fs.FS, string) ([]finding.Finding, error) { return f.findings, f.err }

func TestCatalogCheckReadsSchemaConsts(t *testing.T) {
	var evs []catalog.Event
	repo := fstest.MapFS{}
	for _, e := range catalog.Architecture62 {
		file := catalog.ExpectedSchemaFile(e.Name, 1)
		evs = append(evs, catalog.Event{Name: e.Name, Version: 1, Producer: e.Producer, Consumers: e.Consumers,
			SchemaFile: file, Channel: catalog.ExpectedChannel(e.Producer, e.Name, 1)})
		producer := e.Producer
		if e.Name == "PaymentReceived" {
			producer = "billing" // el schema contradice al catálogo
		}
		repo[file] = &fstest.MapFile{Data: []byte(`{"properties":{"eventType":{"const":"` + e.Name +
			`"},"version":{"const":1},"sourceService":{"const":"` + producer + `"}}}`)}
	}
	got, err := NewCatalogCheck(fakeCatalog{events: evs}, fakeSpec{}).Run(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Rule != "catalog-schema-consts" {
		t.Fatalf("hallazgos = %+v", got)
	}
}

func TestCatalogCheckMissingDocument(t *testing.T) {
	got, err := NewCatalogCheck(fakeCatalog{}, fakeSpec{err: ErrArtifactMissing}).Run(context.Background(), fstest.MapFS{})
	if err != nil || len(got) != 1 || got[0].Rule != "asyncapi-missing" {
		t.Fatalf("hallazgos = %+v, err = %v", got, err)
	}
}

func TestCatalogCheckKeepsSpecFindings(t *testing.T) {
	spec := fakeSpec{findings: []finding.Finding{finding.Errorf("asyncapi-spec", AsyncAPIFile, "", "x")}}
	got, err := NewCatalogCheck(fakeCatalog{}, spec).Run(context.Background(), fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(got, func(f finding.Finding) bool { return f.Rule == "asyncapi-spec" }) ||
		!slices.ContainsFunc(got, func(f finding.Finding) bool { return f.Rule == "catalog-missing-event" }) {
		t.Fatalf("hallazgos = %+v", got)
	}
}
