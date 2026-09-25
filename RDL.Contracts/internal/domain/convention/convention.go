// Package convention convierte docs/convenciones.md en reglas verificables sobre los artefactos (JSON Schema,
// OpenAPI y el registro de problem types). Cada regla es una función pura: agregar una es agregarla a su lista.
package convention

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"

	"bitbucket.org/rdl/contracts/internal/domain/jsondoc"
)

type Violation struct {
	Rule    string
	Pointer string
	Message string
}

var (
	camelCase = regexp.MustCompile(`^[a-z][a-zA-Z0-9]*$`)
	kebabCase = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	// Nombres que en este dominio siempre son dinero o decimales (convenciones §3).
	decimalName = regexp.MustCompile(`(?i)(amount|total|subtotal|tax|discount|price|balance|exoneration|rate|quantity|percentage)$`)
)

// schemaNode visita cada subschema con su puntero. Recorre properties, items, allOf/anyOf/oneOf, if/then/else, not
// y $defs. En OpenAPI también se usa sobre components/schemas y los schemas embebidos en parámetros y cuerpos.
func schemaNodes(root any, base string, visit func(node map[string]any, ptr string)) {
	var walk func(v any, ptr string)
	walk = func(v any, ptr string) {
		n, ok := v.(map[string]any)
		if !ok {
			return
		}
		visit(n, ptr)
		for _, k := range slices.Sorted(maps.Keys(asMap(n["properties"]))) {
			walk(asMap(n["properties"])[k], ptr+"/properties/"+jsondoc.Escape(k))
		}
		for _, k := range slices.Sorted(maps.Keys(asMap(n["$defs"]))) {
			walk(asMap(n["$defs"])[k], ptr+"/$defs/"+jsondoc.Escape(k))
		}
		walk(n["items"], ptr+"/items")
		for _, key := range []string{"if", "then", "else", "not"} {
			walk(n[key], ptr+"/"+key)
		}
		for _, key := range []string{"allOf", "anyOf", "oneOf"} {
			for i, s := range asSlice(n[key]) {
				walk(s, fmt.Sprintf("%s/%s/%d", ptr, key, i))
			}
		}
	}
	walk(root, base)
}

// SchemaRules aplica las reglas de datos a un subárbol de schema (un archivo de schemas/ o un schema de OpenAPI).
func SchemaRules(root any, base string) []Violation {
	var out []Violation
	add := func(rule, ptr, format string, args ...any) {
		out = append(out, Violation{rule, ptr, fmt.Sprintf(format, args...)})
	}
	schemaNodes(root, base, func(n map[string]any, ptr string) {
		if hasType(n, "number") {
			add("no-number", ptr, "type number prohibido: los decimales viajan como string (convenciones §3)")
		}
		for _, name := range slices.Sorted(maps.Keys(asMap(n["properties"]))) {
			prop, _ := asMap(n["properties"])[name].(map[string]any)
			pptr := ptr + "/properties/" + jsondoc.Escape(name)
			if !camelCase.MatchString(name) {
				add("camel-case", pptr, "%q no es camelCase (convenciones §4)", name)
			}
			if prop == nil || isConstOnly(prop) {
				continue
			}
			ref, _ := prop["$ref"].(string)
			switch {
			case strings.HasSuffix(name, "At"):
				if !strings.HasSuffix(ref, "utc-datetime.json") && !strings.HasSuffix(ref, "/UtcDateTime") && prop["format"] != "date-time" {
					add("instant-format", pptr, "%s es un instante: use UtcDateTime (RFC 3339 con Z)", name)
				}
			case strings.HasSuffix(name, "Date") || (strings.HasSuffix(name, "On") && len(name) > 2):
				if !strings.HasSuffix(ref, "business-date.json") && !strings.HasSuffix(ref, "/BusinessDate") && prop["format"] != "date" {
					add("date-format", pptr, "%s es una fecha de negocio: use BusinessDate (YYYY-MM-DD)", name)
				}
			case decimalName.MatchString(name) && hasType(prop, "integer"):
				add("decimal-as-integer", pptr, "%s parece un monto o decimal: use Money, Quantity, Percentage o ExchangeRate", name)
			}
		}
	})
	return out
}

// ClosedObjects: en los schemas de eventos, todo objeto con properties se cierra (additionalProperties o
// unevaluatedProperties en false), salvo los schemas base que otros componen con allOf (x-rdl-base: true).
func ClosedObjects(root any) []Violation {
	var out []Violation
	if r, ok := root.(map[string]any); ok && r["x-rdl-base"] == true {
		return nil
	}
	schemaNodes(root, "", func(n map[string]any, ptr string) {
		if n["properties"] == nil || isInsideComposition(ptr) {
			return
		}
		if n["additionalProperties"] != false && n["unevaluatedProperties"] != false {
			out = append(out, Violation{"closed-object", ptr,
				"objeto abierto: agregue additionalProperties: false (o unevaluatedProperties: false si compone con allOf)"})
		}
	})
	return out
}

// isInsideComposition: las ramas de if/then/else/not y los allOf solo agregan restricciones al objeto que las contiene.
func isInsideComposition(ptr string) bool {
	for _, k := range []string{"/if", "/then", "/else", "/not", "/allOf/"} {
		if strings.Contains(ptr, k) {
			return true
		}
	}
	return false
}

// EventRules: un schema de evento compone el sobre, fija eventType/version/sourceService con const y se cierra con
// unevaluatedProperties: false.
func EventRules(root any) []Violation {
	r, _ := root.(map[string]any)
	var out []Violation
	add := func(rule, ptr, msg string) { out = append(out, Violation{rule, ptr, msg}) }
	composesEnvelope := slices.ContainsFunc(asSlice(r["allOf"]), func(s any) bool {
		ref, _ := asMap(s)["$ref"].(string)
		return strings.HasSuffix(ref, "envelope.v1.json")
	})
	if !composesEnvelope {
		add("event-envelope", "/allOf", "el evento debe componer el sobre: allOf: [{$ref: envelope.v1.json}]")
	}
	if r["unevaluatedProperties"] != false {
		add("event-closed", "/unevaluatedProperties", "el evento debe cerrar con unevaluatedProperties: false")
	}
	for _, k := range []string{"eventType", "version", "sourceService"} {
		p, _ := asMap(r["properties"])[k].(map[string]any)
		if _, ok := p["const"]; !ok {
			add("event-const", "/properties/"+k, k+" debe fijarse con const")
		}
	}
	return out
}

// OpenAPIRules: tenancy, versión en la ruta e idempotencia en los POST.
func OpenAPIRules(doc any) []Violation {
	var out []Violation
	add := func(rule, ptr, format string, args ...any) {
		out = append(out, Violation{rule, ptr, fmt.Sprintf(format, args...)})
	}
	paths := asMap(asMap(doc)["paths"])
	for _, p := range slices.Sorted(maps.Keys(paths)) {
		pptr := "/paths/" + jsondoc.Escape(p)
		if !strings.HasPrefix(p, "/v1/") && !strings.HasPrefix(p, "/internal/v1/") && p != "/healthz" && p != "/readyz" {
			add("openapi-path-version", pptr, "%s: las rutas llevan la versión mayor (/v1/ o /internal/v1/)", p)
		}
		item := asMap(paths[p])
		params := asSlice(item["parameters"])
		for _, m := range []string{"get", "put", "post", "delete", "patch"} {
			op, ok := item[m].(map[string]any)
			if !ok {
				continue
			}
			optr := pptr + "/" + m
			all := append(slices.Clone(params), asSlice(op["parameters"])...)
			for i, prm := range all {
				name, _ := asMap(prm)["name"].(string)
				ref, _ := asMap(prm)["$ref"].(string)
				if tenantParam(name) || tenantParam(ref) {
					add("openapi-tenant-param", fmt.Sprintf("%s/parameters/%d", optr, i),
						"la organización sale del token, nunca de un parámetro (convenciones §5)")
				}
			}
			if m == "post" && !strings.HasPrefix(p, "/internal/") && !slices.ContainsFunc(all, isIdempotencyKey) {
				add("openapi-idempotency", optr, "POST %s sin Idempotency-Key (convenciones §7)", p)
			}
		}
	}
	return out
}

func tenantParam(s string) bool {
	l := strings.ToLower(s)
	return strings.Contains(l, "organizationid") || strings.Contains(l, "organization_id") || strings.Contains(l, "tenant")
}

func isIdempotencyKey(prm any) bool {
	m := asMap(prm)
	ref, _ := m["$ref"].(string)
	name, _ := m["name"].(string)
	return strings.HasSuffix(ref, "/IdempotencyKey") || strings.EqualFold(name, "Idempotency-Key")
}

// Problem es una entrada de problems/<servicio>.yaml.
type Problem struct {
	Code   string
	Status int
	// UpstreamStatus (`status: upstream` en el YAML): el tipo conserva el status que respondió otra API. Solo
	// lo usa un servicio de borde, que compone APIs ajenas; ninguna API de dominio lo necesita.
	UpstreamStatus bool
	Title          string
	When           string
}

// CommonProblems son los tipos que toda API expone (convenciones §5-§7).
var CommonProblems = []string{
	"unauthenticated", "no-active-organization", "membership-inactive", "forbidden", "not-found",
	"malformed-request", "validation", "conflict", "idempotency-key-required", "idempotency-key-reused", "internal",
}

// EdgeProblems son los servicios de borde y el mínimo que cada uno registra. Publican problem types pero no son
// servicios de la base (no están en ownership.Services ni en los CHECK de audit e integration). El Portal
// Gateway no revalida membresías ni valida cuerpos: esos errores los da la API dueña y él los reenvía intactos,
// así que no se le exigen los tipos comunes de una API de dominio.
var EdgeProblems = map[string][]string{
	"portal-gateway": {"unauthenticated", "not-found", "method-not-allowed", "internal"},
}

// ProblemRules valida el registro de un servicio.
func ProblemRules(service string, problems []Problem) []Violation {
	var out []Violation
	add := func(rule, ptr, format string, args ...any) {
		out = append(out, Violation{rule, ptr, fmt.Sprintf(format, args...)})
	}
	seen := map[string]bool{}
	for i, p := range problems {
		ptr := fmt.Sprintf("/problems/%d", i)
		if !kebabCase.MatchString(p.Code) {
			add("problem-code", ptr+"/code", "%q no es kebab-case", p.Code)
		}
		if seen[p.Code] {
			add("problem-duplicate", ptr+"/code", "%q repetido", p.Code)
		}
		seen[p.Code] = true
		_, edge := EdgeProblems[service]
		switch {
		case p.UpstreamStatus && !edge:
			add("problem-status", ptr+"/status", "%s: solo un servicio de borde conserva el status de otra API", p.Code)
		case !p.UpstreamStatus && (p.Status < 400 || p.Status > 599):
			add("problem-status", ptr+"/status", "%s: status %d fuera de 4xx/5xx", p.Code, p.Status)
		}
		if strings.TrimSpace(p.Title) == "" || strings.TrimSpace(p.When) == "" {
			add("problem-docs", ptr, "%s necesita title y when", p.Code)
		}
	}
	required := CommonProblems
	if edge, ok := EdgeProblems[service]; ok {
		required = edge
	}
	for _, c := range required {
		if !seen[c] {
			add("problem-common", "/problems", "%s debe registrar el tipo común %q (urn:rdl:%s:problem:%s)", service, c, service, c)
		}
	}
	return out
}

func hasType(n map[string]any, t string) bool {
	switch v := n["type"].(type) {
	case string:
		return v == t
	case []any:
		return slices.Contains(v, any(t))
	}
	return false
}

func isConstOnly(n map[string]any) bool { _, ok := n["const"]; return ok && len(n) == 1 }

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asSlice(v any) []any {
	s, _ := v.([]any)
	return s
}
