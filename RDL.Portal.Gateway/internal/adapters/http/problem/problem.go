// Package problem escribe los errores PROPIOS del Portal Gateway como Problem Details (RFC 9457).
//
// Los errores de las APIs no pasan por aquí: se reenvían sin cambios (mismo status, mismo `type`, mismo cuerpo),
// porque el portal ya sabe interpretarlos y reescribirlos escondería de quién fue el problema.
// Aquí solo nacen los del gateway: API caída, timeout, ruta no declarada.
package problem

import (
	"encoding/json"
	"net/http"

	"rdl/portal-gateway/pkg/correlation"
)

const ContentType = "application/problem+json"

// TypeBase es el prefijo de los `type` de este servicio.
// TODO(contracts): `problems/portal-gateway.yaml` está propuesto en docs/propuestas/; falta el PR al repo de
// contratos (2 aprobaciones + CHANGELOG). Mientras tanto, los códigos de aquí son la propuesta.
const TypeBase = "urn:rdl:portal-gateway:problem:"

type Details struct {
	Type          string `json:"type"`
	Title         string `json:"title"`
	Status        int    `json:"status"`
	Detail        string `json:"detail,omitempty"`
	Instance      string `json:"instance,omitempty"`
	CorrelationID string `json:"correlationId,omitempty"`
}

// New construye un problema con un `type` estable a partir de un slug (ej. "upstream-unavailable").
func New(status int, slug, title string) Details {
	return Details{Type: TypeBase + slug, Title: title, Status: status}
}

func (d Details) WithDetail(detail string) Details {
	d.Detail = detail
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

// Catálogo del Portal Gateway. Los detalles internos (host, error de conexión) NUNCA van en Detail:
// van al log. Detail solo dice qué servicio y qué pasó, en términos del portal.
var (
	Internal     = New(http.StatusInternalServerError, "internal", "Error interno")
	NotFound     = New(http.StatusNotFound, "not-found", "Ruta no encontrada")
	MethodNot    = New(http.StatusMethodNotAllowed, "method-not-allowed", "Método no permitido")
	Unauthorized = New(http.StatusUnauthorized, "unauthenticated", "Autenticación requerida")
	TooLarge     = New(http.StatusRequestEntityTooLarge, "payload-too-large", "El cuerpo excede el máximo permitido")

	// UpstreamUnavailable: la API dueña no respondió (caída, DNS, conexión rechazada, 5xx sin cuerpo útil).
	UpstreamUnavailable = New(http.StatusBadGateway, "upstream-unavailable", "El servicio no está disponible")
	// UpstreamTimeout: la API dueña no respondió a tiempo.
	UpstreamTimeout = New(http.StatusGatewayTimeout, "upstream-timeout", "El servicio tardó demasiado en responder")
	// UpstreamNotConfigured: la API todavía no está desplegada en este ambiente (E-Invoice y Receivables hoy).
	// Es distinto de "caída": le dice al portal que la función no existe todavía, no que se rompió.
	UpstreamNotConfigured = New(http.StatusServiceUnavailable, "upstream-not-configured",
		"El servicio todavía no está disponible en este ambiente")
)
