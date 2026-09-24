// Package jsonpatch aplica el subconjunto de JSON Patch (RFC 6902) que usan las suites de ejemplos: add, remove y
// replace sobre JSON Pointers (RFC 6901). Así un ejemplo inválido se escribe como "el válido 0 con total como number"
// en lugar de copiar el evento entero.
package jsonpatch

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type Op struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

var ErrInvalidPatch = errors.New("patch inválido")

// Apply devuelve una copia del documento con las operaciones aplicadas; el original no cambia.
func Apply(doc any, ops []Op) (any, error) {
	out := clone(doc)
	for i, op := range ops {
		tokens, err := parsePointer(op.Path)
		if err != nil {
			return nil, fmt.Errorf("%w: operación %d: %w", ErrInvalidPatch, i, err)
		}
		if len(tokens) == 0 {
			return nil, fmt.Errorf("%w: operación %d: no se puede modificar la raíz", ErrInvalidPatch, i)
		}
		if out, err = applyAt(out, tokens, op); err != nil {
			return nil, fmt.Errorf("%w: operación %d (%s %s): %w", ErrInvalidPatch, i, op.Op, op.Path, err)
		}
	}
	return out, nil
}

func applyAt(node any, tokens []string, op Op) (any, error) {
	key, rest := tokens[0], tokens[1:]
	switch n := node.(type) {
	case map[string]any:
		if len(rest) > 0 {
			child, ok := n[key]
			if !ok {
				return nil, fmt.Errorf("no existe %q", key)
			}
			updated, err := applyAt(child, rest, op)
			if err != nil {
				return nil, err
			}
			n[key] = updated
			return n, nil
		}
		_, exists := n[key]
		switch op.Op {
		case "add":
			n[key] = clone(op.Value)
		case "replace":
			if !exists {
				return nil, fmt.Errorf("replace sobre %q, que no existe", key)
			}
			n[key] = clone(op.Value)
		case "remove":
			if !exists {
				return nil, fmt.Errorf("remove sobre %q, que no existe", key)
			}
			delete(n, key)
		default:
			return nil, fmt.Errorf("operación %q no soportada (add, remove, replace)", op.Op)
		}
		return n, nil
	case []any:
		if len(rest) == 0 && op.Op == "add" && key == "-" {
			return append(n, clone(op.Value)), nil
		}
		idx, err := strconv.Atoi(key)
		if err != nil || idx < 0 || idx >= len(n) {
			return nil, fmt.Errorf("índice %q fuera de rango", key)
		}
		if len(rest) > 0 {
			updated, err := applyAt(n[idx], rest, op)
			if err != nil {
				return nil, err
			}
			n[idx] = updated
			return n, nil
		}
		switch op.Op {
		case "replace":
			n[idx] = clone(op.Value)
			return n, nil
		case "remove":
			return append(n[:idx], n[idx+1:]...), nil
		case "add":
			n = append(n[:idx], append([]any{clone(op.Value)}, n[idx:]...)...)
			return n, nil
		}
		return nil, fmt.Errorf("operación %q no soportada (add, remove, replace)", op.Op)
	}
	return nil, fmt.Errorf("%q no es un objeto ni un arreglo", key)
}

func parsePointer(p string) ([]string, error) {
	if p == "" {
		return nil, nil
	}
	if !strings.HasPrefix(p, "/") {
		return nil, fmt.Errorf("el path %q debe empezar con /", p)
	}
	parts := strings.Split(p[1:], "/")
	for i, t := range parts {
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(t, "~1", "/"), "~0", "~")
	}
	return parts, nil
}

func clone(v any) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, x := range t {
			m[k] = clone(x)
		}
		return m
	case []any:
		s := make([]any, len(t))
		for i, x := range t {
			s[i] = clone(x)
		}
		return s
	}
	return v
}
