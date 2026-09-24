package yamlfs

import (
	"errors"
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"

	"bitbucket.org/rdl/contracts/internal/app"
)

// DocumentLoader lee un YAML o JSON (JSON es YAML válido) como árbol genérico: map[string]any, []any y escalares.
type DocumentLoader struct{}

func (DocumentLoader) Load(repo fs.FS, file string) (any, error) {
	raw, err := fs.ReadFile(repo, file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, app.ErrArtifactMissing
	}
	if err != nil {
		return nil, fmt.Errorf("leyendo %s: %w", file, err)
	}
	var doc any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%w: %w", app.ErrArtifactMalformed, err)
	}
	return doc, nil
}
