// Package http traduce HTTP ↔ casos de uso. No contiene reglas de negocio: el Portal Gateway pide, junta y
// entrega. Ninguna ruta existe fuera de `internal/domain/routes`.
package http

import (
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"rdl/portal-gateway/internal/domain/routes"
	"rdl/portal-gateway/internal/platform/health"
	"rdl/portal-gateway/pkg/correlation"
)

type Deps struct {
	Log      *slog.Logger
	Health   *health.Handler
	Verifier Verifier
	Proxy    *Proxy
	Overview *OverviewHandler
	Invoices *InvoiceListHandler
	Budget   time.Duration
	CORS     []string
	// Service es el nombre del span raíz.
	Service string
}

// NewRouter arma el mux recorriendo la tabla de rutas. Nada de reverse proxy abierto: una ruta que no está
// en la tabla es un 404 y nunca llega a ninguna API.
//
//	/healthz, /readyz   públicas
//	/portal/v1/...      requieren un JWT válido, verificado aquí
func NewRouter(d Deps) (http.Handler, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", d.Health.Live)
	mux.HandleFunc("GET /readyz", d.Health.Ready)

	portal := http.NewServeMux()
	portal.HandleFunc("/", notFound)
	if err := register(portal, d); err != nil {
		return nil, err
	}

	var protected http.Handler = portal
	protected = authenticate(d.Verifier, d.Log)(protected)
	protected = withBudget(d.Budget)(protected)
	mux.Handle("/portal/v1/", protected)
	mux.HandleFunc("/", notFound)

	var h http.Handler = mux
	h = accessLog(d.Log, h)
	h = cors(d.CORS)(h)
	h = recoverer(d.Log, h)
	h = correlation.Middleware(h)
	return otelhttp.NewHandler(h, d.Service), nil
}

// register vuelca la tabla en el mux: UNA inscripción por ruta, que despacha por método y responde 405 en
// Problem Details cuando la ruta existe pero el método no.
//
// El método no va en el patrón a propósito. El mux de Go rechaza como ambiguo un patrón literal sin método
// (`/portal/v1/catalogs/cabys`, que haría falta para el 405) frente a uno con método y comodín
// (`GET /portal/v1/catalogs/{catalog}`). Sin método en el patrón, manda la precedencia normal —lo literal
// gana al comodín— y no hay conflicto.
func register(mux *http.ServeMux, d Deps) error {
	type entry struct {
		handlers map[string]http.HandlerFunc
		allow    []string
	}
	byPath := map[string]*entry{}
	var order []string

	for _, route := range routes.Table() {
		handler, err := handlerFor(route, d)
		if err != nil {
			return err
		}
		e, ok := byPath[route.Path]
		if !ok {
			e = &entry{handlers: map[string]http.HandlerFunc{}}
			byPath[route.Path] = e
			order = append(order, route.Path)
		}
		if _, dup := e.handlers[route.Method]; dup {
			return fmt.Errorf("la tabla declara dos veces %s %s", route.Method, route.Path)
		}
		e.handlers[route.Method] = handler
		e.allow = append(e.allow, route.Method)
	}

	for _, path := range order {
		e := byPath[path]
		slices.Sort(e.allow)
		allow := strings.Join(e.allow, ", ")
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			handler, ok := e.handlers[r.Method]
			if !ok {
				w.Header().Set("Allow", allow)
				methodNotAllowed(w, r)
				return
			}
			handler(w, r)
		})
	}
	return nil
}

func handlerFor(route routes.Route, d Deps) (http.HandlerFunc, error) {
	switch route.Kind {
	case routes.Passthrough:
		return d.Proxy.Handler(route), nil
	case routes.Composed:
		return composedHandler(route, d)
	default:
		return nil, fmt.Errorf("ruta %s %s: tipo desconocido %q", route.Method, route.Path, route.Kind)
	}
}

// composedHandler conecta cada vista con su caso de uso. Agregar una composición es agregar su fila en la
// tabla y su caso aquí: si falta uno de los dos, el gateway no arranca en lugar de servir una ruta a medias.
func composedHandler(route routes.Route, d Deps) (http.HandlerFunc, error) {
	key := route.Method + " " + route.Path
	switch {
	case key == "GET /portal/v1/invoices/{id}/overview" && d.Overview != nil:
		return d.Overview.Get, nil
	case key == "GET /portal/v1/invoices" && d.Invoices != nil:
		return d.Invoices.List, nil
	}
	return nil, fmt.Errorf("la composición %s está declarada en la tabla pero no tiene caso de uso", key)
}

// RegisteredRoutes lista las rutas que el gateway expone. Lo usan el arranque (para el log) y las pruebas.
func RegisteredRoutes() []string {
	table := routes.Table()
	out := make([]string, 0, len(table))
	for _, r := range table {
		out = append(out, strings.TrimSpace(r.Method+" "+r.Path))
	}
	return out
}
