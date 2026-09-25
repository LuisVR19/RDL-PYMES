// Package problem escribe errores HTTP como Problem Details (RFC 9457).
// Origen: RDL.Platform.API, copiado sin cambios de comportamiento (docs/decisiones/0003-reutilizacion-de-platform.md).
package problem

import (
	"encoding/json"
	"net/http"

	"rdl/receivables-api/pkg/correlation"
)

const ContentType = "application/problem+json"

// TypeBase es el prefijo de los `type` estables (RDL.Contracts/docs/convenciones.md §6, problems/receivables.yaml).
const TypeBase = "urn:rdl:receivables:problem:"

type Details struct {
	Type          string       `json:"type"`
	Title         string       `json:"title"`
	Status        int          `json:"status"`
	Detail        string       `json:"detail,omitempty"`
	Instance      string       `json:"instance,omitempty"`
	CorrelationID string       `json:"correlationId,omitempty"`
	Errors        []FieldError `json:"errors,omitempty"`
}

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// New construye un problema con un `type` estable a partir de un slug (ej. "not-found").
func New(status int, slug, title string) Details {
	return Details{Type: TypeBase + slug, Title: title, Status: status}
}

func (d Details) WithDetail(detail string) Details {
	d.Detail = detail
	return d
}

func (d Details) WithErrors(errs ...FieldError) Details {
	d.Errors = errs
	return d
}

// Write completa instance y correlationId desde el request y serializa la respuesta.
func Write(w http.ResponseWriter, r *http.Request, d Details) {
	if d.Instance == "" {
		d.Instance = r.URL.Path
	}
	if id, ok := correlation.FromContext(r.Context()); ok {
		d.CorrelationID = id.String()
	}
	w.Header().Set("Content-Type", ContentType)
	w.WriteHeader(d.Status)
	_ = json.NewEncoder(w).Encode(d)
}

var (
	Internal         = New(http.StatusInternalServerError, "internal", "Error interno")
	NotFound         = New(http.StatusNotFound, "not-found", "Recurso no encontrado")
	MethodNotAllowed = New(http.StatusMethodNotAllowed, "method-not-allowed", "Método no permitido")
	Unauthorized     = New(http.StatusUnauthorized, "unauthenticated", "Autenticación requerida")
	Forbidden        = New(http.StatusForbidden, "forbidden", "Permisos insuficientes")
	Unavailable      = New(http.StatusServiceUnavailable, "unavailable", "Servicio no disponible")
)
