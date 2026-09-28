package http

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// api/openapi.yaml y el router tienen exactamente las mismas operaciones: un endpoint nuevo sin documentar (o una
// ruta documentada sin handler) hace fallar este test.
func TestOpenAPIMatchesRoutes(t *testing.T) {
	raw, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	param := regexp.MustCompile(`\{[^}]+\}`)
	var spec []string
	for path, ops := range doc.Paths {
		for method := range ops {
			switch method {
			case "get", "post", "put", "patch", "delete":
				spec = append(spec, strings.ToUpper(method)+" "+param.ReplaceAllString(path, "{}"))
			}
		}
	}

	v1, internal := newRoutes(), newRoutes()
	(&ReceivableHandlers{}).register(v1, nil)
	(&ReceivableHandlers{}).registerInternal(internal, nil)
	(&PaymentHandlers{}).register(v1, nil)
	(&CollectionHandlers{}).register(v1, nil)
	var code []string
	for _, rt := range []*routes{v1, internal} {
		for path, methods := range rt.methods {
			for m := range methods {
				code = append(code, m+" "+param.ReplaceAllString(path, "{}"))
			}
		}
	}
	slices.Sort(spec)
	slices.Sort(code)
	if !slices.Equal(spec, code) {
		t.Errorf("api/openapi.yaml y el router no coinciden:\nspec   %v\nrouter %v", spec, code)
	}
}
