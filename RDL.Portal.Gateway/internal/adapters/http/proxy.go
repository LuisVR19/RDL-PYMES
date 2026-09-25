package http

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"rdl/portal-gateway/internal/adapters/downstream"
	"rdl/portal-gateway/internal/adapters/http/problem"
	"rdl/portal-gateway/internal/app"
	"rdl/portal-gateway/internal/domain/routes"
)

// Proxy ejecuta el PASO DIRECTO: mismo método, mismo cuerpo, mismos errores. No mira el cuerpo, no lo
// reinterpreta y no agrega nada. Solo existe para una ruta declarada en la tabla.
type Proxy struct {
	Clients map[routes.Service]*downstream.Client
	MaxBody int64
	Log     *slog.Logger
}

// Handler arma el handler de una fila de la tabla.
func (p *Proxy) Handler(route routes.Route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		client := p.Clients[route.Service]
		if !client.Configured() {
			// TODO(P5/P6): desaparece cuando E-Invoice y Receivables estén desplegadas.
			problem.Write(w, r, problem.UpstreamNotConfigured.
				WithDetail("La función correspondiente todavía no está habilitada."))
			return
		}

		body := io.Reader(nil)
		if r.Body != nil && route.IsCommand() {
			body = http.MaxBytesReader(w, r.Body, p.MaxBody)
		}

		resp, err := client.Send(r.Context(), downstream.Request{
			Method: route.Method,
			Path:   expand(route.Upstream, r),
			Query:  cleanQuery(r.URL.Query()),
			Header: forwardHeaders(r),
			Body:   body,
			// Solo las lecturas se reintentan. Un comando lo reintenta el cliente con su Idempotency-Key.
			Retryable: !route.IsCommand(),
		})
		if err != nil {
			p.writeUpstreamError(w, r, route, err)
			return
		}
		defer func() { _ = resp.Body.Close() }()

		copyHeaders(w.Header(), resp.Header, routes.ResponseHeaders)
		w.WriteHeader(resp.StatusCode)
		// El cuerpo se copia tal cual, incluidos los Problem Details de la API: el portal ya los entiende
		// y reescribirlos escondería de quién fue el problema.
		if _, err := io.Copy(w, resp.Body); err != nil {
			p.Log.WarnContext(r.Context(), "se cortó la copia de la respuesta",
				slog.String("upstream", string(route.Service)), slog.Any("error", err))
		}
	}
}

func (p *Proxy) writeUpstreamError(w http.ResponseWriter, r *http.Request, route routes.Route, err error) {
	var maxBytes *http.MaxBytesError
	switch {
	case errors.As(err, &maxBytes):
		problem.Write(w, r, problem.TooLarge)
		return
	case errors.Is(err, app.ErrNotConfigured):
		problem.Write(w, r, problem.UpstreamNotConfigured)
		return
	}

	// El detalle (host, error de conexión) se queda en el log: al portal solo le sirve saber qué pasó.
	p.Log.ErrorContext(r.Context(), "la API destino no respondió",
		slog.String("upstream", string(route.Service)),
		slog.String("route", route.Method+" "+route.Path),
		slog.Any("error", err))

	if errors.Is(err, app.ErrTimeout) {
		problem.Write(w, r, problem.UpstreamTimeout)
		return
	}
	problem.Write(w, r, problem.UpstreamUnavailable)
}

// expand sustituye los comodines de la plantilla destino con los valores de la ruta pública, escapados.
func expand(template string, r *http.Request) string {
	out := template
	for _, name := range routes.Params(template) {
		out = strings.ReplaceAll(out, "{"+name+"}", url.PathEscape(r.PathValue(name)))
	}
	return out
}

// forwardHeaders aplica la lista blanca: lo que no está declarado no viaja. El X-Correlation-Id es el que
// fijó el gateway, no el que mandó el cliente (que solo se respeta si era un UUID).
func forwardHeaders(r *http.Request) http.Header {
	out := http.Header{}
	copyHeaders(out, r.Header, routes.RequestHeaders)
	if id := correlationOf(r); id != "" {
		out.Set("X-Correlation-Id", id)
	}
	// Authorization lo pone el cliente de salida con el token verificado del contexto, no se copia de aquí:
	// así es imposible reenviar un encabezado que no pasó por la verificación.
	out.Del("Authorization")
	return out
}

func copyHeaders(dst, src http.Header, allowed []string) {
	for _, name := range allowed {
		for _, v := range src.Values(name) {
			dst.Add(name, v)
		}
	}
}

// cleanQuery borra cualquier intento de mandar la organización por la query. El Portal Gateway no la agrega
// y tampoco la deja pasar: la organización sale del token y la resuelve cada API contra su base.
func cleanQuery(q url.Values) url.Values {
	for _, p := range routes.StrippedQueryParams {
		q.Del(p)
	}
	return q
}
