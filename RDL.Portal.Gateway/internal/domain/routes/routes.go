// Package routes declara la ÚNICA tabla de rutas del Portal Gateway. Agregar una pantalla es agregar filas aquí,
// nunca escribir otro proxy: el router recorre esta tabla y no expone nada que no esté declarado.
// Es dominio: datos y reglas de forma, sin net/http ni ningún cliente.
package routes

import "strings"

// Service es la API dueña de la operación. El Portal Gateway no decide nada de negocio: solo a quién preguntarle.
type Service string

const (
	Platform    Service = "platform"
	Billing     Service = "billing"
	Fiscal      Service = "fiscal"
	Receivables Service = "receivables"
)

// Services son los cuatro servicios en el orden en que se documentan. Fiscal y Receivables están declarados
// desde el día uno aunque sus repos todavía no existan: una ruta que aparece después obliga a reabrir el router,
// la configuración, los health checks y las pruebas (ADR 0002).
func Services() []Service { return []Service{Platform, Billing, Fiscal, Receivables} }

// Kind distingue reenviar de componer.
type Kind string

const (
	// Passthrough reenvía a una sola API: mismo método, mismo cuerpo, mismos errores.
	Passthrough Kind = "passthrough"
	// Composed junta varias APIs en una vista. Su handler se registra aparte, pero la ruta se declara igual aquí.
	Composed Kind = "composed"
)

// Route es una fila de la tabla.
type Route struct {
	Method string
	// Path es la ruta pública, con los comodines del router de Go 1.22+ (`{id}`).
	Path string
	Kind Kind
	// Service es la API destino. Vacío en Composed: el caso de uso sabe a quiénes llama.
	Service Service
	// Upstream es la plantilla en la API destino. Sus `{nombre}` tienen que existir en Path.
	Upstream string
	// Why documenta la pantalla o la capacidad del portal que la usa (docs/decisiones/0001).
	Why string
}

// IsCommand: todo lo que no es lectura. Un comando nunca se reintenta y propaga Idempotency-Key.
func (r Route) IsCommand() bool { return r.Method != "GET" && r.Method != "HEAD" }

// PathParams devuelve los nombres de los comodines de Upstream, en orden de aparición.
func (r Route) PathParams() []string { return Params(r.Upstream) }

// Params devuelve los nombres de los comodines `{nombre}` de un patrón, en orden de aparición.
func Params(pattern string) []string {
	var out []string
	rest := pattern
	for {
		_, after, found := strings.Cut(rest, "{")
		if !found {
			return out
		}
		name, tail, closed := strings.Cut(after, "}")
		if !closed {
			return out
		}
		out = append(out, name)
		rest = tail
	}
}

// RequestHeaders es la lista blanca de lo que viaja del portal a las APIs. Todo lo demás se descarta.
//
//	Authorization    el token del usuario, byte por byte: nunca un token de servicio.
//	X-Correlation-Id lo fija el middleware de correlación (el del cliente solo se respeta si es un UUID).
//	Idempotency-Key  el reintento del cliente tiene que llegar con la misma llave a la API dueña.
//	Accept-Language  mensajes de las APIs en el idioma del usuario.
//	Accept,
//	Content-Type     sin ellos la API no sabe interpretar ni negociar el cuerpo.
//	traceparent,
//	tracestate       contexto de trazas W3C. El propagador de OpenTelemetry los reescribe cuando hay un span
//	                 activo; se copian para no perder la traza cuando no hay SDK configurado (local sin collector).
var RequestHeaders = []string{
	"Authorization",
	"X-Correlation-Id",
	"Idempotency-Key",
	"Accept-Language",
	"Accept",
	"Content-Type",
	"traceparent",
	"tracestate",
}

// ResponseHeaders es la lista blanca de vuelta. Nada de cabeceras internas de las APIs hacia el navegador.
var ResponseHeaders = []string{
	"Content-Type",
	"Content-Language",
	"X-Correlation-Id",
	"Location",
	"Retry-After",
	"ETag",
}

// InvoiceListParams son los filtros del listado que viajan a Billing (los de su GET /v1/invoices). Una
// composición no reenvía la query entera: pasa solo lo declarado aquí, así que una organización nunca viaja.
var InvoiceListParams = []string{
	"limit", "cursor", "documentType", "status", "customerId", "requiresCorrection", "issuedFrom", "issuedTo",
}

// StrippedQueryParams se borran de la query antes de reenviar. El Portal Gateway no agrega una organización,
// y tampoco deja que el cliente intente colarla: la organización sale del token y la resuelve cada API.
var StrippedQueryParams = []string{"organizationId", "organization_id", "orgId", "org_id"}

// Table es la tabla completa. El orden es el de docs/decisiones/0001-inventario-pantallas-y-apis.md.
//
// Fase actual: Platform y Billing tienen repo; Fiscal y Receivables están declarados contra los esqueletos de
// `openapi/fiscal.yaml` y `openapi/receivables.yaml` del repo de contratos. Sin su URL configurada, esas filas
// responden 503 `upstream-not-configured` y las composiciones degradan (ADR 0004).
func Table() []Route {
	var t []Route
	t = append(t, platformRoutes()...)
	t = append(t, billingRoutes()...)
	t = append(t, fiscalRoutes()...)
	t = append(t, receivablesRoutes()...)
	t = append(t, composedRoutes()...)
	return t
}

// composedRoutes son las vistas que el Portal Gateway arma con varias APIs (arquitectura 2.2).
func composedRoutes() []Route {
	return []Route{
		{Method: "GET", Path: "/portal/v1/invoices/{id}/overview", Kind: Composed,
			Why: "Pantalla 15 · Detalle de factura: total de Billing + estado de Hacienda + saldo"},
	}
}

func platformRoutes() []Route {
	const s = Platform
	return []Route{
		{Method: "GET", Path: "/portal/v1/me", Kind: Passthrough, Service: s,
			Upstream: "/v1/me", Why: "Armazón · sesión y perfil (pantalla 33)"},
		{Method: "GET", Path: "/portal/v1/me/memberships", Kind: Passthrough, Service: s,
			Upstream: "/v1/me/memberships", Why: "Pantalla 2 · Selector de organización"},
		{Method: "PUT", Path: "/portal/v1/me/active-organization", Kind: Passthrough, Service: s,
			Upstream: "/v1/me/active-organization", Why: "Pantalla 5 · Cambio de organización"},

		{Method: "POST", Path: "/portal/v1/organizations", Kind: Passthrough, Service: s,
			Upstream: "/v1/organizations", Why: "Pantalla 3 · Crear organización"},
		{Method: "GET", Path: "/portal/v1/organizations/current", Kind: Passthrough, Service: s,
			Upstream: "/v1/organizations/current", Why: "Pantalla 28 · Organización"},
		{Method: "PATCH", Path: "/portal/v1/organizations/current", Kind: Passthrough, Service: s,
			Upstream: "/v1/organizations/current", Why: "Pantalla 28 · Organización"},

		{Method: "GET", Path: "/portal/v1/organizations/current/users", Kind: Passthrough, Service: s,
			Upstream: "/v1/organizations/current/users", Why: "Pantalla 30 · Usuarios y roles"},
		{Method: "PATCH", Path: "/portal/v1/organizations/current/users/{userId}", Kind: Passthrough, Service: s,
			Upstream: "/v1/organizations/current/users/{userId}", Why: "Pantalla 30 · Usuarios y roles"},

		{Method: "GET", Path: "/portal/v1/organizations/current/invitations", Kind: Passthrough, Service: s,
			Upstream: "/v1/organizations/current/invitations", Why: "Pantalla 31 · Invitaciones"},
		{Method: "POST", Path: "/portal/v1/organizations/current/invitations", Kind: Passthrough, Service: s,
			Upstream: "/v1/organizations/current/invitations", Why: "Pantalla 31 · Invitaciones"},
		{Method: "DELETE", Path: "/portal/v1/organizations/current/invitations/{id}", Kind: Passthrough, Service: s,
			Upstream: "/v1/organizations/current/invitations/{id}", Why: "Pantalla 31 · Invitaciones"},
		{Method: "POST", Path: "/portal/v1/invitations/{token}/accept", Kind: Passthrough, Service: s,
			Upstream: "/v1/invitations/{token}/accept", Why: "Pantalla 4 · Aceptar invitación"},

		{Method: "GET", Path: "/portal/v1/organizations/current/branches", Kind: Passthrough, Service: s,
			Upstream: "/v1/organizations/current/branches", Why: "Pantalla 29 · Sucursales"},
		{Method: "POST", Path: "/portal/v1/organizations/current/branches", Kind: Passthrough, Service: s,
			Upstream: "/v1/organizations/current/branches", Why: "Pantalla 29 · Sucursales"},
		{Method: "GET", Path: "/portal/v1/organizations/current/branches/{id}", Kind: Passthrough, Service: s,
			Upstream: "/v1/organizations/current/branches/{id}", Why: "Pantalla 29 · Sucursales"},
		{Method: "PATCH", Path: "/portal/v1/organizations/current/branches/{id}", Kind: Passthrough, Service: s,
			Upstream: "/v1/organizations/current/branches/{id}", Why: "Pantalla 29 · Sucursales"},
	}
}

func billingRoutes() []Route {
	const s = Billing
	return []Route{
		{Method: "GET", Path: "/portal/v1/customers", Kind: Passthrough, Service: s,
			Upstream: "/v1/customers", Why: "Pantalla 7 · Clientes"},
		{Method: "POST", Path: "/portal/v1/customers", Kind: Passthrough, Service: s,
			Upstream: "/v1/customers", Why: "Pantalla 8 · Nuevo cliente"},
		{Method: "GET", Path: "/portal/v1/customers/{id}", Kind: Passthrough, Service: s,
			Upstream: "/v1/customers/{id}", Why: "Pantalla 9 · Ficha del cliente"},
		{Method: "PATCH", Path: "/portal/v1/customers/{id}", Kind: Passthrough, Service: s,
			Upstream: "/v1/customers/{id}", Why: "Pantalla 8 · Editar cliente"},

		{Method: "GET", Path: "/portal/v1/products", Kind: Passthrough, Service: s,
			Upstream: "/v1/products", Why: "Pantalla 10 · Productos y servicios"},
		{Method: "POST", Path: "/portal/v1/products", Kind: Passthrough, Service: s,
			Upstream: "/v1/products", Why: "Pantalla 11 · Nuevo producto"},
		{Method: "GET", Path: "/portal/v1/products/{id}", Kind: Passthrough, Service: s,
			Upstream: "/v1/products/{id}", Why: "Pantalla 11 · Editar producto"},
		{Method: "PATCH", Path: "/portal/v1/products/{id}", Kind: Passthrough, Service: s,
			Upstream: "/v1/products/{id}", Why: "Pantalla 11 · Editar producto"},

		{Method: "GET", Path: "/portal/v1/invoices", Kind: Composed,
			Why: "Pantalla 12 · Documentos: página de Billing + estado de Hacienda + saldo, una llamada por API"},
		{Method: "POST", Path: "/portal/v1/invoices", Kind: Passthrough, Service: s,
			Upstream: "/v1/invoices", Why: "Pantalla 13 · Nueva factura"},
		{Method: "GET", Path: "/portal/v1/invoices/{id}", Kind: Passthrough, Service: s,
			Upstream: "/v1/invoices/{id}", Why: "Pantalla 15 · Detalle de factura"},
		{Method: "PATCH", Path: "/portal/v1/invoices/{id}", Kind: Passthrough, Service: s,
			Upstream: "/v1/invoices/{id}", Why: "Pantalla 13 · Editar borrador"},
		{Method: "DELETE", Path: "/portal/v1/invoices/{id}", Kind: Passthrough, Service: s,
			Upstream: "/v1/invoices/{id}", Why: "Pantalla 13 · Descartar borrador"},
		{Method: "PUT", Path: "/portal/v1/invoices/{id}/lines", Kind: Passthrough, Service: s,
			Upstream: "/v1/invoices/{id}/lines", Why: "Pantalla 13 · Líneas del borrador"},
		{Method: "POST", Path: "/portal/v1/invoices/{id}/issue", Kind: Passthrough, Service: s,
			Upstream: "/v1/invoices/{id}/issue", Why: "Pantalla 14 · Emitir"},
		{Method: "GET", Path: "/portal/v1/invoices/{id}/history", Kind: Passthrough, Service: s,
			Upstream: "/v1/invoices/{id}/history", Why: "Pantalla 15 · Historial de estados"},

		{Method: "GET", Path: "/portal/v1/document-sequences", Kind: Passthrough, Service: s,
			Upstream: "/v1/document-sequences", Why: "Pantalla 28 · Numeración"},
		{Method: "PUT", Path: "/portal/v1/document-sequences/{documentType}", Kind: Passthrough, Service: s,
			Upstream: "/v1/document-sequences/{documentType}", Why: "Pantalla 28 · Numeración"},
	}
}

// fiscalRoutes salen de `openapi/fiscal.yaml` del repo de contratos (esqueleto). TODO(P5): confirmarlas contra
// RDL.EInvoice.API cuando exista; los campos de sus respuestas son `TODO(fiscal)` en el contrato.
func fiscalRoutes() []Route {
	const s = Fiscal
	return []Route{
		{Method: "GET", Path: "/portal/v1/fiscal-profile", Kind: Passthrough, Service: s,
			Upstream: "/v1/fiscal-profile", Why: "Pantalla 18 · Configuración fiscal"},
		{Method: "PUT", Path: "/portal/v1/fiscal-profile", Kind: Passthrough, Service: s,
			Upstream: "/v1/fiscal-profile", Why: "Pantalla 18 · Configuración fiscal"},
		{Method: "PUT", Path: "/portal/v1/fiscal-profile/certificate", Kind: Passthrough, Service: s,
			Upstream: "/v1/fiscal-profile/certificate", Why: "Pantalla 18 · Certificado de firma"},
		{Method: "GET", Path: "/portal/v1/establishments", Kind: Passthrough, Service: s,
			Upstream: "/v1/establishments", Why: "Pantalla 19 · Establecimientos y terminales"},
		{Method: "GET", Path: "/portal/v1/electronic-documents", Kind: Passthrough, Service: s,
			Upstream: "/v1/electronic-documents", Why: "Pantalla 20 · Bandeja"},
		{Method: "GET", Path: "/portal/v1/electronic-documents/{id}", Kind: Passthrough, Service: s,
			Upstream: "/v1/electronic-documents/{id}", Why: "Pantalla 21 · Documento electrónico"},
		{Method: "GET", Path: "/portal/v1/electronic-documents/{id}/files/{kind}", Kind: Passthrough, Service: s,
			Upstream: "/v1/electronic-documents/{id}/files/{kind}", Why: "Pantalla 21 · XML y respuesta de Hacienda"},
		{Method: "POST", Path: "/portal/v1/electronic-documents/{id}/retry", Kind: Passthrough, Service: s,
			Upstream: "/v1/electronic-documents/{id}/retry", Why: "Pantalla 21 · Reintentar envío"},
		{Method: "GET", Path: "/portal/v1/catalogs/cabys", Kind: Passthrough, Service: s,
			Upstream: "/v1/catalogs/cabys", Why: "Pantalla 11 · Buscador de CABYS"},
		{Method: "GET", Path: "/portal/v1/catalogs/{catalog}", Kind: Passthrough, Service: s,
			Upstream: "/v1/catalogs/{catalog}", Why: "Pantallas 11 y 18 · Catálogos fiscales"},
	}
}

// receivablesRoutes salen de `openapi/receivables.yaml` del repo de contratos (esqueleto).
// TODO(P6): confirmarlas contra RDL.Receivables.API cuando exista.
func receivablesRoutes() []Route {
	const s = Receivables
	return []Route{
		{Method: "GET", Path: "/portal/v1/receivables", Kind: Passthrough, Service: s,
			Upstream: "/v1/receivables", Why: "Pantalla 22 · Cuentas por cobrar"},
		{Method: "GET", Path: "/portal/v1/receivables/aging", Kind: Passthrough, Service: s,
			Upstream: "/v1/receivables/aging", Why: "Pantalla 23 · Aging"},
		{Method: "GET", Path: "/portal/v1/receivables/{id}", Kind: Passthrough, Service: s,
			Upstream: "/v1/receivables/{id}", Why: "Pantalla 24 · Detalle de cuenta"},
		{Method: "GET", Path: "/portal/v1/receivables/{id}/follow-ups", Kind: Passthrough, Service: s,
			Upstream: "/v1/receivables/{id}/follow-ups", Why: "Pantalla 24 · Gestiones de cobro"},
		{Method: "GET", Path: "/portal/v1/payments", Kind: Passthrough, Service: s,
			Upstream: "/v1/payments", Why: "Pantalla 25 · Pagos"},
		{Method: "POST", Path: "/portal/v1/payments", Kind: Passthrough, Service: s,
			Upstream: "/v1/payments", Why: "Pantalla 26 · Registrar pago"},
		{Method: "GET", Path: "/portal/v1/payments/{id}", Kind: Passthrough, Service: s,
			Upstream: "/v1/payments/{id}", Why: "Pantalla 27 · Detalle de pago"},
		{Method: "POST", Path: "/portal/v1/payments/{id}/void", Kind: Passthrough, Service: s,
			Upstream: "/v1/payments/{id}/void", Why: "Pantalla 27 · Anular pago"},
		{Method: "POST", Path: "/portal/v1/payment-applications", Kind: Passthrough, Service: s,
			Upstream: "/v1/payment-applications", Why: "Pantalla 26 · Aplicar pago a documentos"},
		{Method: "POST", Path: "/portal/v1/payment-applications/{id}/reverse", Kind: Passthrough, Service: s,
			Upstream: "/v1/payment-applications/{id}/reverse", Why: "Pantalla 27 · Revertir aplicación"},
	}
}
