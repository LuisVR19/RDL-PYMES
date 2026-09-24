// Package ownership modela la matriz de la tabla 2.1 de la arquitectura: qué servicio es dueño de cada schema y qué
// puede leer o escribir cada uno en los ajenos. Las reglas son funciones puras sobre ese modelo.
package ownership

import (
	"fmt"
	"slices"
	"strings"
)

// SharedOwner es el dueño de los schemas transversales (audit, integration, shared): el repo database-platform.
const SharedOwner = "database-platform"

// Services son los servicios que la base reconoce en sus CHECK (audit, integration, idempotency_keys).
var Services = []string{"platform", "billing", "fiscal", "receivables"}

type Op string

const (
	Insert Op = "insert"
	Update Op = "update"
	Delete Op = "delete"
)

type Service struct {
	Name         string
	DisplayName  string
	AppRole      string
	MigratorRole string
}

// Access es lo que un servicio puede hacer en un schema que no es suyo.
type Access struct {
	Service string
	// ReadAll: lee todas las tablas. Si es false, solo ReadTables (lectura explícita, arquitectura 2.1).
	ReadAll    bool
	ReadTables []string
	Write      []Op
	Note       string
}

type Schema struct {
	Name    string
	Owner   string
	Purpose string
	Access  []Access
	// AppendOnly: ningún servicio puede actualizar ni borrar (audit, arquitectura 7.3).
	AppendOnly bool
}

type Matrix struct {
	Services []Service
	Schemas  []Schema
}

// Violation es una regla incumplida; Pointer apunta a la clave del YAML (JSON Pointer).
type Violation struct {
	Rule    string
	Pointer string
	Message string
}

// Violations aplica todas las invariantes. El orden de la salida es el del documento.
func (m Matrix) Violations() []Violation {
	var out []Violation
	add := func(rule, ptr, format string, args ...any) {
		out = append(out, Violation{Rule: rule, Pointer: ptr, Message: fmt.Sprintf(format, args...)})
	}

	known := map[string]bool{}
	for _, s := range m.Services {
		ptr := "/services/" + s.Name
		known[s.Name] = true
		if !slices.Contains(Services, s.Name) {
			add("ownership-unknown-service", ptr, "servicio %q desconocido: la base solo admite %s", s.Name, strings.Join(Services, ", "))
		}
		if s.AppRole != s.Name+"_app" {
			add("ownership-role-name", ptr+"/appRole", "el rol de aplicación debe ser %q, no %q", s.Name+"_app", s.AppRole)
		}
		if s.MigratorRole != s.Name+"_migrator" {
			add("ownership-role-name", ptr+"/migratorRole", "el rol migrador debe ser %q, no %q", s.Name+"_migrator", s.MigratorRole)
		}
	}
	for _, name := range Services {
		if !known[name] {
			add("ownership-missing-service", "/services", "falta el servicio %q", name)
		}
	}

	for _, sc := range m.Schemas {
		ptr := "/schemas/" + sc.Name
		if sc.Owner != SharedOwner && !known[sc.Owner] {
			add("ownership-unknown-owner", ptr+"/owner", "dueño %q desconocido (un servicio o %q)", sc.Owner, SharedOwner)
		}
		sharedSchema := sc.Owner == SharedOwner
		for _, a := range sc.Access {
			aptr := ptr + "/access/" + a.Service
			if !known[a.Service] {
				add("ownership-unknown-service", aptr, "servicio %q desconocido", a.Service)
			}
			if a.Service == sc.Owner {
				add("ownership-owner-in-access", aptr, "el dueño no se declara en access: ya tiene todos los permisos")
			}
			if a.ReadAll && len(a.ReadTables) > 0 {
				add("ownership-read-ambiguous", aptr+"/read", "read es `all` o una lista de tablas, no ambas")
			}
			// Una API nunca escribe en el schema de otra API (arquitectura 2.1). Solo en los transversales.
			if len(a.Write) > 0 && !sharedSchema {
				add("ownership-foreign-write", aptr+"/write", "%s no puede escribir en %s, que es de %s", a.Service, sc.Name, sc.Owner)
			}
			if sc.AppendOnly && (slices.Contains(a.Write, Update) || slices.Contains(a.Write, Delete)) {
				add("ownership-append-only", aptr+"/write", "%s es append only: solo se permite insert", sc.Name)
			}
			for _, op := range a.Write {
				if op != Insert && op != Update && op != Delete {
					add("ownership-unknown-op", aptr+"/write", "operación %q desconocida (insert, update, delete)", op)
				}
			}
		}
	}

	for _, name := range Services {
		if !slices.ContainsFunc(m.Schemas, func(s Schema) bool { return s.Owner == name }) {
			add("ownership-service-without-schema", "/services/"+name, "%s no es dueño de ningún schema", name)
		}
	}
	return out
}
