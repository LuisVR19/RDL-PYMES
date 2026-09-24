// Package finding modela un hallazgo de validación: qué regla falló, en qué archivo y en qué punto del documento.
// Es el lenguaje común entre los validadores del dominio y la salida del CLI.
package finding

import (
	"cmp"
	"slices"
)

type Severity string

const (
	// Error hace fallar el comando (y el CI).
	Error Severity = "error"
	// Warning se informa pero no hace fallar: sirve para TODOs conocidos, como los campos fiscales pendientes.
	Warning Severity = "warning"
)

type Finding struct {
	Severity Severity `json:"severity"`
	Rule     string   `json:"rule"`
	File     string   `json:"file"`
	// Pointer es un JSON Pointer (RFC 6901) dentro del archivo; vacío si el hallazgo es del archivo entero.
	Pointer string `json:"pointer,omitempty"`
	Message string `json:"message"`
}

func Errorf(rule, file, pointer, message string) Finding {
	return Finding{Severity: Error, Rule: rule, File: file, Pointer: pointer, Message: message}
}

func Warnf(rule, file, pointer, message string) Finding {
	return Finding{Severity: Warning, Rule: rule, File: file, Pointer: pointer, Message: message}
}

// Sort ordena de forma estable (archivo, puntero, regla, mensaje) para que la salida del CI sea reproducible.
func Sort(fs []Finding) {
	slices.SortStableFunc(fs, func(a, b Finding) int {
		return cmp.Or(
			cmp.Compare(a.File, b.File),
			cmp.Compare(a.Pointer, b.Pointer),
			cmp.Compare(a.Rule, b.Rule),
			cmp.Compare(a.Message, b.Message),
		)
	})
}

func HasErrors(fs []Finding) bool {
	return slices.ContainsFunc(fs, func(f Finding) bool { return f.Severity == Error })
}
