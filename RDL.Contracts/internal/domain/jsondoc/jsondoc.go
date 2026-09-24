// Package jsondoc trabaja sobre documentos JSON/YAML genéricos (map[string]any, []any): recoge los $ref y resuelve
// JSON Pointers (RFC 6901). Lo usan las verificaciones de OpenAPI y AsyncAPI para encontrar referencias rotas.
package jsondoc

import (
	"maps"
	"slices"
	"strconv"
	"strings"
)

// Ref es un $ref encontrado en Pointer (dónde está) que apunta a Target ("archivo#/puntero", "#/puntero" o "archivo").
type Ref struct {
	Pointer string
	Target  string
}

// Refs recorre el documento en orden estable y devuelve todos los $ref.
func Refs(doc any) []Ref {
	var out []Ref
	var walk func(v any, ptr string)
	walk = func(v any, ptr string) {
		switch t := v.(type) {
		case map[string]any:
			if r, ok := t["$ref"].(string); ok {
				out = append(out, Ref{Pointer: ptr, Target: r})
			}
			for _, k := range slices.Sorted(maps.Keys(t)) {
				walk(t[k], ptr+"/"+Escape(k))
			}
		case []any:
			for i, x := range t {
				walk(x, ptr+"/"+strconv.Itoa(i))
			}
		}
	}
	walk(doc, "")
	return out
}

// Split separa "archivo#/puntero" en sus partes. Un target sin '#' apunta al documento entero.
func Split(target string) (file, pointer string) {
	file, pointer, _ = strings.Cut(target, "#")
	return file, pointer
}

// Resolve sigue un JSON Pointer; "" es el documento entero.
func Resolve(doc any, pointer string) (any, bool) {
	if pointer == "" {
		return doc, true
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, false
	}
	cur := doc
	for _, tok := range strings.Split(pointer[1:], "/") {
		tok = Unescape(tok)
		switch t := cur.(type) {
		case map[string]any:
			next, ok := t[tok]
			if !ok {
				return nil, false
			}
			cur = next
		case []any:
			i, err := strconv.Atoi(tok)
			if err != nil || i < 0 || i >= len(t) {
				return nil, false
			}
			cur = t[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

func Escape(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1") }
func Unescape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
}
