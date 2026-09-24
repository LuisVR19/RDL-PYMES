// Package catalog verifica el catálogo de eventos (AsyncAPI) contra la tabla 6.2 de la arquitectura y contra los
// schemas: ningún evento fuera del catálogo (D8), productor y consumidores acordados, y nombres de archivo y canal
// derivados del nombre del evento.
package catalog

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Expected es una fila de la tabla 6.2.
type Expected struct {
	Name      string
	Producer  string
	Consumers []string
}

// Architecture62 es la tabla 6.2 de docs/contexto/arquitectura-v1.md. Agregar un evento aquí exige cambiar primero
// la arquitectura (y dos aprobaciones).
var Architecture62 = []Expected{
	{"InvoiceIssued", "billing", []string{"fiscal", "receivables"}},
	{"InvoiceCancelled", "billing", []string{"fiscal", "receivables"}},
	{"CreditNoteIssued", "billing", []string{"fiscal", "receivables"}},
	{"DebitNoteIssued", "billing", []string{"fiscal", "receivables"}},
	{"ElectronicDocumentAccepted", "fiscal", []string{"bff", "billing", "notifications"}},
	{"ElectronicDocumentRejected", "fiscal", []string{"bff", "billing", "notifications"}},
	{"PaymentReceived", "receivables", []string{"bff", "reports"}},
	{"ReceivableSettled", "receivables", []string{"bff", "reports"}},
}

// Event es un mensaje del documento AsyncAPI.
type Event struct {
	Name       string
	Version    int
	Producer   string
	Consumers  []string
	SchemaFile string
	Channel    string
	// Pointer ubica el mensaje en el documento (para los hallazgos).
	Pointer string
}

// SchemaConsts son los const que fija el schema del evento. Found es false si el archivo no existe.
type SchemaConsts struct {
	Found         bool
	EventType     string
	Version       int
	SourceService string
}

type Violation struct {
	Rule    string
	Pointer string
	Message string
}

var upper = regexp.MustCompile(`([a-z0-9])([A-Z])`)

// Kebab convierte InvoiceIssued en invoice-issued.
func Kebab(name string) string { return strings.ToLower(upper.ReplaceAllString(name, "$1-$2")) }

func ExpectedSchemaFile(name string, version int) string {
	return fmt.Sprintf("schemas/events/%s.v%d.json", Kebab(name), version)
}

func ExpectedChannel(producer, name string, version int) string {
	return fmt.Sprintf("%s.%s.v%d", producer, Kebab(name), version)
}

// Check aplica todas las reglas. consts se indexa por SchemaFile.
func Check(events []Event, expected []Expected, consts map[string]SchemaConsts) []Violation {
	var out []Violation
	add := func(rule, ptr, format string, args ...any) {
		out = append(out, Violation{Rule: rule, Pointer: ptr, Message: fmt.Sprintf(format, args...)})
	}

	byName := map[string]Expected{}
	for _, e := range expected {
		byName[e.Name] = e
	}
	seen := map[string]bool{}
	hasV1 := map[string]bool{}
	for _, ev := range events {
		key := fmt.Sprintf("%s v%d", ev.Name, ev.Version)
		if seen[key] {
			add("catalog-duplicate", ev.Pointer, "%s aparece dos veces", key)
		}
		seen[key] = true

		exp, ok := byName[ev.Name]
		if !ok {
			add("catalog-unknown-event", ev.Pointer, "%s no está en la tabla 6.2 de la arquitectura: no se agregan eventos (D8)", ev.Name)
			continue
		}
		if ev.Version < 1 {
			add("catalog-version", ev.Pointer+"/x-rdl-version", "versión %d inválida", ev.Version)
		}
		if ev.Version == 1 {
			hasV1[ev.Name] = true
		}
		if ev.Producer != exp.Producer {
			add("catalog-producer", ev.Pointer+"/x-rdl-producer", "%s lo produce %s, no %s", ev.Name, exp.Producer, ev.Producer)
		}
		if !sameSet(ev.Consumers, exp.Consumers) {
			add("catalog-consumers", ev.Pointer+"/x-rdl-consumers", "consumidores de %s: %v (arquitectura 6.2), no %v", ev.Name, exp.Consumers, ev.Consumers)
		}
		if want := ExpectedSchemaFile(ev.Name, ev.Version); ev.SchemaFile != want {
			add("catalog-schema-file", ev.Pointer+"/payload/schema", "el schema de %s debe ser %s, no %s", key, want, ev.SchemaFile)
		}
		if want := ExpectedChannel(exp.Producer, ev.Name, ev.Version); ev.Channel != want {
			add("catalog-channel", ev.Pointer, "el canal de %s debe tener address %q, tiene %q", key, want, ev.Channel)
		}

		c := consts[ev.SchemaFile]
		switch {
		case !c.Found:
			add("catalog-schema-missing", ev.Pointer+"/payload/schema", "no existe %s", ev.SchemaFile)
		case c.EventType != ev.Name || c.Version != ev.Version || c.SourceService != exp.Producer:
			add("catalog-schema-consts", ev.Pointer+"/payload/schema",
				"%s fija eventType=%q, version=%d, sourceService=%q; se esperaba %q, %d, %q",
				ev.SchemaFile, c.EventType, c.Version, c.SourceService, ev.Name, ev.Version, exp.Producer)
		}
	}
	for _, e := range expected {
		if !hasV1[e.Name] {
			add("catalog-missing-event", "/components/messages", "falta %s v1 (tabla 6.2 de la arquitectura)", e.Name)
		}
	}
	return out
}

func sameSet(a, b []string) bool {
	x, y := slices.Clone(a), slices.Clone(b)
	slices.Sort(x)
	slices.Sort(y)
	return slices.Equal(x, y)
}
