// Package openapi extrae las operaciones de un documento OpenAPI (modelo genérico) y aplica reglas propias del repo:
// toda operación tiene operationId único y los comandos de las máquinas de estado existen en la API dueña.
package openapi

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"bitbucket.org/rdl/contracts/internal/domain/jsondoc"
)

var methods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

type Operation struct {
	Method  string // en mayúsculas: GET, POST...
	Path    string
	ID      string
	Pointer string
}

// Key es "MÉTODO /ruta", el mismo formato que los comandos de las máquinas de estado.
func (o Operation) Key() string { return o.Method + " " + o.Path }

// Operations devuelve las operaciones en orden estable.
func Operations(doc any) []Operation {
	root, _ := doc.(map[string]any)
	paths, _ := root["paths"].(map[string]any)
	var out []Operation
	for _, p := range slices.Sorted(maps.Keys(paths)) {
		item, _ := paths[p].(map[string]any)
		for _, m := range methods {
			op, ok := item[m].(map[string]any)
			if !ok {
				continue
			}
			id, _ := op["operationId"].(string)
			out = append(out, Operation{
				Method: strings.ToUpper(m), Path: p, ID: id,
				Pointer: "/paths/" + jsondoc.Escape(p) + "/" + m,
			})
		}
	}
	return out
}

type Violation struct {
	Rule    string
	Pointer string
	Message string
}

// OperationViolations: operationId obligatorio y único dentro del documento (los generadores de clientes lo usan).
func OperationViolations(ops []Operation) []Violation {
	var out []Violation
	seen := map[string]string{}
	for _, o := range ops {
		switch {
		case o.ID == "":
			out = append(out, Violation{"openapi-operation-id", o.Pointer, o.Key() + " no tiene operationId"})
		case seen[o.ID] != "":
			out = append(out, Violation{"openapi-operation-id", o.Pointer + "/operationId",
				fmt.Sprintf("operationId %q repetido (también en %s)", o.ID, seen[o.ID])})
		default:
			seen[o.ID] = o.Key()
		}
	}
	return out
}
