package yamlfs

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"

	"gopkg.in/yaml.v3"

	"bitbucket.org/rdl/contracts/internal/app"
	"bitbucket.org/rdl/contracts/internal/domain/catalog"
)

// AsyncAPILoader lee los mensajes del documento AsyncAPI y sus extensiones x-rdl-*. La estructura completa la valida
// el meta-schema oficial (adapters/jsonschema); aquí solo se extrae lo que usa el catálogo.
type AsyncAPILoader struct{}

type asyncDoc struct {
	Channels map[string]struct {
		Address  string `yaml:"address"`
		Messages map[string]struct {
			Ref string `yaml:"$ref"`
		} `yaml:"messages"`
	} `yaml:"channels"`
	Components struct {
		Messages map[string]struct {
			Name    string `yaml:"name"`
			Payload struct {
				Schema struct {
					Ref string `yaml:"$ref"`
				} `yaml:"schema"`
			} `yaml:"payload"`
			Version   int      `yaml:"x-rdl-version"`
			Producer  string   `yaml:"x-rdl-producer"`
			Consumers []string `yaml:"x-rdl-consumers"`
		} `yaml:"messages"`
	} `yaml:"components"`
}

func (AsyncAPILoader) Load(repo fs.FS, file string) ([]catalog.Event, error) {
	raw, err := fs.ReadFile(repo, file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, app.ErrArtifactMissing
	}
	if err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", file, err)
	}
	var doc asyncDoc
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%w: %w", app.ErrArtifactMalformed, err)
	}

	channelOf := map[string]string{}
	for _, ch := range doc.Channels {
		for _, m := range ch.Messages {
			channelOf[m.Ref] = ch.Address
		}
	}
	var out []catalog.Event
	for _, key := range slices.Sorted(maps.Keys(doc.Components.Messages)) {
		m := doc.Components.Messages[key]
		schemaFile := ""
		if m.Payload.Schema.Ref != "" {
			schemaFile = path.Join(path.Dir(file), m.Payload.Schema.Ref)
		}
		out = append(out, catalog.Event{
			Name: m.Name, Version: m.Version, Producer: m.Producer, Consumers: m.Consumers,
			SchemaFile: schemaFile, Channel: channelOf["#/components/messages/"+key],
			Pointer: "/components/messages/" + key,
		})
	}
	return out, nil
}
