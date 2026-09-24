// Package compat decide qué cambios entre dos versiones de un contrato son incompatibles (docs/convenciones.md §11).
// Es conservador: ante la duda (un pattern distinto, un $ref que apunta a otro lado) el cambio cuenta como
// incompatible, porque un falso positivo cuesta una revisión y un falso negativo rompe a un consumidor.
package compat

import (
	"fmt"
	"maps"
	"reflect"
	"slices"

	"bitbucket.org/rdl/contracts/internal/domain/jsondoc"
)

type Change struct {
	Rule    string
	Pointer string
	Message string
}

// Schemas compara dos versiones de un JSON Schema y devuelve los cambios incompatibles.
func Schemas(old, cur any) []Change {
	var out []Change
	compareNode(asMap(old), asMap(cur), "", &out)
	return out
}

func compareNode(o, n map[string]any, ptr string, out *[]Change) {
	add := func(rule, p, format string, args ...any) {
		*out = append(*out, Change{rule, p, fmt.Sprintf(format, args...)})
	}
	if o == nil {
		return
	}
	if n == nil {
		add("schema-removed", ptr, "el subschema desapareció")
		return
	}
	for _, k := range []string{"type", "$ref", "const", "format", "pattern", "additionalProperties", "unevaluatedProperties"} {
		ov, oOK := o[k]
		nv, nOK := n[k]
		if oOK != nOK || !reflect.DeepEqual(ov, nv) {
			add(k+"-changed", ptr+"/"+jsondoc.Escape(k), "%s cambió de %s a %s", k, show(ov, oOK), show(nv, nOK))
		}
	}

	// enum: quitar un valor rompe a quien lo envía; agregarlo rompe a quien lo consume (los eventos no son extensibles).
	if oe, ne := asSlice(o["enum"]), asSlice(n["enum"]); oe != nil || ne != nil {
		for _, v := range oe {
			if !slices.ContainsFunc(ne, func(x any) bool { return reflect.DeepEqual(x, v) }) {
				add("enum-value-removed", ptr+"/enum", "se quitó el valor %v", v)
			}
		}
		for _, v := range ne {
			if !slices.ContainsFunc(oe, func(x any) bool { return reflect.DeepEqual(x, v) }) {
				add("enum-value-added", ptr+"/enum", "se agregó el valor %v: los consumidores actuales no lo conocen", v)
			}
		}
	}

	// Límites: endurecer rompe; aflojar no.
	for _, k := range []string{"minLength", "minimum", "exclusiveMinimum", "minItems"} {
		if tighter(o[k], n[k], func(a, b float64) bool { return b > a }) {
			add("bound-tightened", ptr+"/"+k, "%s pasó de %s a %s", k, show(o[k], o[k] != nil), show(n[k], true))
		}
	}
	for _, k := range []string{"maxLength", "maximum", "exclusiveMaximum", "maxItems"} {
		if tighter(o[k], n[k], func(a, b float64) bool { return b < a }) {
			add("bound-tightened", ptr+"/"+k, "%s pasó de %s a %s", k, show(o[k], o[k] != nil), show(n[k], true))
		}
	}

	oldReq, newReq := stringList(o["required"]), stringList(n["required"])
	for _, r := range newReq {
		if !slices.Contains(oldReq, r) {
			add("required-added", ptr+"/required", "%s pasó a ser obligatorio", r)
		}
	}

	op, np := asMap(o["properties"]), asMap(n["properties"])
	for _, name := range slices.Sorted(maps.Keys(op)) {
		pp := ptr + "/properties/" + jsondoc.Escape(name)
		if _, ok := np[name]; !ok {
			add("property-removed", pp, "se quitó el campo %s", name)
			continue
		}
		compareNode(asMap(op[name]), asMap(np[name]), pp, out)
	}

	compareNode(asMap(o["items"]), asMap(n["items"]), ptr+"/items", out)
	for _, k := range []string{"if", "then", "else", "not"} {
		compareNode(asMap(o[k]), asMap(n[k]), ptr+"/"+k, out)
		if o[k] == nil && n[k] != nil {
			add(k+"-added", ptr+"/"+k, "se agregó %s: puede rechazar payloads que hoy son válidos", k)
		}
	}
	for _, k := range []string{"allOf", "anyOf", "oneOf"} {
		oa, na := asSlice(o[k]), asSlice(n[k])
		if len(oa) != len(na) {
			add(k+"-changed", ptr+"/"+k, "%s pasó de %d a %d elementos", k, len(oa), len(na))
			continue
		}
		for i := range oa {
			compareNode(asMap(oa[i]), asMap(na[i]), fmt.Sprintf("%s/%s/%d", ptr, k, i), out)
		}
	}
}

// Param es un parámetro ya resuelto de una operación.
type Param struct {
	Name, In string
	Required bool
}

// Operation es una operación OpenAPI con sus parámetros resueltos.
type Operation struct {
	Key         string // "MÉTODO /ruta"
	Pointer     string
	Params      []Param
	BodyRequire bool
}

// Operations compara dos versiones de las operaciones de una API.
func Operations(old, cur []Operation) []Change {
	var out []Change
	byKey := map[string]Operation{}
	for _, o := range cur {
		byKey[o.Key] = o
	}
	for _, o := range old {
		n, ok := byKey[o.Key]
		if !ok {
			out = append(out, Change{"operation-removed", o.Pointer, "se quitó " + o.Key})
			continue
		}
		for _, p := range n.Params {
			if !p.Required {
				continue
			}
			was := slices.ContainsFunc(o.Params, func(x Param) bool { return x.Name == p.Name && x.In == p.In && x.Required })
			if !was {
				out = append(out, Change{"parameter-required-added", n.Pointer + "/parameters",
					fmt.Sprintf("%s: el parámetro %s (%s) pasó a ser obligatorio", o.Key, p.Name, p.In)})
			}
		}
		if n.BodyRequire && !o.BodyRequire {
			out = append(out, Change{"body-required-added", n.Pointer + "/requestBody", o.Key + ": el cuerpo pasó a ser obligatorio"})
		}
	}
	return out
}

// tighter compara límites de JSON Schema (maxLength, minimum...). Son números de la especificación, no montos: aquí
// float64 es correcto y la regla "sin float" (que aplica a dinero y DTOs) no corresponde.
func tighter(old, cur any, stricter func(a, b float64) bool) bool {
	n, ok := num(cur)
	if !ok {
		return false
	}
	o, ok := num(old)
	if !ok {
		return true // antes no había límite
	}
	return stricter(o, n)
}

func num(v any) (float64, bool) {
	switch t := v.(type) {
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case float64:
		return t, true
	}
	return 0, false
}

func show(v any, present bool) string {
	if !present {
		return "(ausente)"
	}
	return fmt.Sprintf("%v", v)
}

func stringList(v any) []string {
	var out []string
	for _, x := range asSlice(v) {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}
