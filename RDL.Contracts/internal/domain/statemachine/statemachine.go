// Package statemachine modela las máquinas de estado del contrato (factura, documento electrónico, cuenta por cobrar,
// pago) y sus invariantes: estados alcanzables, finales sin salida, disparadores tipados y eventos coherentes con el
// catálogo (el dueño emite solo lo que produce y reacciona solo a lo que consume).
package statemachine

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"bitbucket.org/rdl/contracts/internal/domain/catalog"
	"bitbucket.org/rdl/contracts/internal/domain/ownership"
)

type TriggerKind string

const (
	Command TriggerKind = "command" // petición HTTP a la API dueña
	Event   TriggerKind = "event"   // evento del catálogo que la API dueña consume
	Worker  TriggerKind = "worker"  // proceso interno de la API dueña (firma, envío, reintentos)
)

// DocumentTypes son los valores de schemas/common/document-type.json.
var DocumentTypes = []string{"invoice", "credit_note", "debit_note"}

type Trigger struct {
	Kind TriggerKind
	Name string
}

type State struct {
	Name        string
	Description string
	Final       bool
}

type Transition struct {
	From, To      string
	DocumentTypes []string
	Trigger       Trigger
	Emits         string
	Requires      []string
}

// Creation es cómo nace la entidad: estado inicial, disparador principal y alternativos.
type Creation struct {
	State   string
	Trigger Trigger
	AlsoBy  []Trigger
	Emits   string
}

type Flag struct {
	Name        string
	Description string
	SetBy       Trigger
}

type Machine struct {
	Entity        string
	Owner         string
	Table         string
	Column        string
	DocumentTypes []string
	Created       Creation
	States        []State // en el orden del documento
	Transitions   []Transition
	Flags         []Flag
	TODO          []string
}

type Violation struct {
	Rule    string
	Pointer string
	Message string
}

var commandRe = regexp.MustCompile(`^(GET|POST|PUT|PATCH|DELETE) /v1/\S+$`)

func (m Machine) state(name string) (State, bool) {
	i := slices.IndexFunc(m.States, func(s State) bool { return s.Name == name })
	if i < 0 {
		return State{}, false
	}
	return m.States[i], true
}

// Violations aplica las invariantes contra el catálogo de eventos (tabla 6.2).
func (m Machine) Violations(events []catalog.Expected) []Violation {
	var out []Violation
	add := func(rule, ptr, format string, args ...any) {
		out = append(out, Violation{Rule: rule, Pointer: ptr, Message: fmt.Sprintf(format, args...)})
	}
	ev := map[string]catalog.Expected{}
	for _, e := range events {
		ev[e.Name] = e
	}
	checkTrigger := func(ptr string, t Trigger) {
		switch t.Kind {
		case Command:
			if !commandRe.MatchString(t.Name) {
				add("sm-command", ptr, "un comando es `MÉTODO /v1/ruta`, no %q", t.Name)
			}
		case Event:
			e, ok := ev[t.Name]
			if !ok {
				add("sm-unknown-event", ptr, "%s no está en el catálogo de eventos", t.Name)
			} else if !slices.Contains(e.Consumers, m.Owner) {
				add("sm-event-not-consumed", ptr, "%s no consume %s (consumidores: %v)", m.Owner, t.Name, e.Consumers)
			}
		case Worker:
			if strings.TrimSpace(t.Name) == "" {
				add("sm-trigger", ptr, "el worker necesita un nombre")
			}
		default:
			add("sm-trigger", ptr, "tipo de disparador %q desconocido (command, event, worker)", t.Kind)
		}
	}
	checkEmits := func(ptr, name string) {
		if name == "" {
			return
		}
		e, ok := ev[name]
		if !ok {
			add("sm-unknown-event", ptr, "%s no está en el catálogo de eventos", name)
		} else if e.Producer != m.Owner {
			add("sm-emits-foreign-event", ptr, "%s lo produce %s, no %s", name, e.Producer, m.Owner)
		}
	}

	if !slices.Contains(ownership.Services, m.Owner) {
		add("sm-owner", "/owner", "dueño %q desconocido", m.Owner)
	}
	for _, dt := range m.DocumentTypes {
		if !slices.Contains(DocumentTypes, dt) {
			add("sm-document-type", "/documentTypes", "tipo de documento %q desconocido", dt)
		}
	}
	if len(m.States) == 0 {
		add("sm-no-states", "/states", "la máquina no tiene estados")
		return out
	}

	if s, ok := m.state(m.Created.State); !ok {
		add("sm-initial", "/created/state", "el estado inicial %q no existe", m.Created.State)
	} else if s.Final {
		add("sm-initial", "/created/state", "el estado inicial no puede ser final")
	}
	checkTrigger("/created/trigger", m.Created.Trigger)
	for i, t := range m.Created.AlsoBy {
		checkTrigger(fmt.Sprintf("/created/alsoBy/%d", i), t)
	}
	checkEmits("/created/emits", m.Created.Emits)

	outgoing := map[string]int{}
	seen := map[string]bool{}
	for i, t := range m.Transitions {
		ptr := fmt.Sprintf("/transitions/%d", i)
		from, okFrom := m.state(t.From)
		if !okFrom {
			add("sm-unknown-state", ptr+"/from", "estado %q inexistente", t.From)
		}
		if _, ok := m.state(t.To); !ok {
			add("sm-unknown-state", ptr+"/to", "estado %q inexistente", t.To)
		}
		if okFrom && from.Final {
			add("sm-final-has-exit", ptr, "%s es final: no puede tener transiciones de salida", t.From)
		}
		if t.From == t.To {
			add("sm-self-loop", ptr, "una transición %s → %s no cambia de estado; no se modela", t.From, t.To)
		}
		for _, dt := range t.DocumentTypes {
			if !slices.Contains(m.DocumentTypes, dt) {
				add("sm-document-type", ptr+"/documentTypes", "%q no es un tipo de documento de esta máquina", dt)
			}
		}
		key := fmt.Sprintf("%s>%s|%s:%s|%v", t.From, t.To, t.Trigger.Kind, t.Trigger.Name, t.DocumentTypes)
		if seen[key] {
			add("sm-duplicate-transition", ptr, "transición repetida %s → %s por %s", t.From, t.To, t.Trigger.Name)
		}
		seen[key] = true
		checkTrigger(ptr+"/trigger", t.Trigger)
		checkEmits(ptr+"/emits", t.Emits)
		outgoing[t.From]++
	}
	for _, f := range m.Flags {
		checkTrigger("/flags/"+f.Name+"/setBy", f.SetBy)
	}

	reach := m.reachable()
	for _, s := range m.States {
		ptr := "/states/" + s.Name
		if !reach[s.Name] {
			add("sm-unreachable", ptr, "%s no es alcanzable desde %s", s.Name, m.Created.State)
		}
		if !s.Final && outgoing[s.Name] == 0 {
			add("sm-dead-end", ptr, "%s no es final y no tiene salida (¿falta final: true?)", s.Name)
		}
		if strings.TrimSpace(s.Description) == "" {
			add("sm-description", ptr, "el estado necesita una descripción")
		}
	}
	return out
}

func (m Machine) reachable() map[string]bool {
	seen := map[string]bool{m.Created.State: true}
	queue := []string{m.Created.State}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, t := range m.Transitions {
			if t.From == cur && !seen[t.To] {
				seen[t.To] = true
				queue = append(queue, t.To)
			}
		}
	}
	return seen
}

// Mermaid dibuja la máquina como stateDiagram-v2. Es determinista: el doc de cada máquina incluye este bloque tal cual
// y `contractsctl validate` avisa si se desincroniza del YAML.
func (m Machine) Mermaid() string {
	var b strings.Builder
	b.WriteString("stateDiagram-v2\n")
	fmt.Fprintf(&b, "    [*] --> %s : %s\n", m.Created.State, label(m.Created.Trigger, m.Created.Emits, nil))
	for _, t := range m.Created.AlsoBy {
		fmt.Fprintf(&b, "    [*] --> %s : %s\n", m.Created.State, label(t, "", nil))
	}
	for _, t := range m.Transitions {
		fmt.Fprintf(&b, "    %s --> %s : %s\n", t.From, t.To, label(t.Trigger, t.Emits, t.DocumentTypes))
	}
	for _, s := range m.States {
		if s.Final {
			fmt.Fprintf(&b, "    %s --> [*]\n", s.Name)
		}
	}
	return b.String()
}

func label(t Trigger, emits string, docTypes []string) string {
	var s string
	switch t.Kind {
	case Command:
		s = t.Name
	case Event:
		s = "evento " + t.Name
	default:
		s = "worker · " + t.Name
	}
	if len(docTypes) > 0 {
		s += " (" + strings.Join(docTypes, ", ") + ")"
	}
	if emits != "" {
		s += " ⇒ " + emits
	}
	return s
}
