// Package downstream son los clientes hacia las APIs dueñas. Aquí vive todo lo que el Portal Gateway hace
// "hacia afuera": reenviar el token del usuario, respetar timeouts, reintentar solo lecturas y traducir
// la respuesta a los errores que entienden los casos de uso.
//
// Ninguna función de este paquete interpreta datos de negocio: copia, decodifica y clasifica.
package downstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"rdl/portal-gateway/internal/app"
	"rdl/portal-gateway/internal/domain/routes"
	"rdl/portal-gateway/pkg/correlation"
	"rdl/portal-gateway/pkg/identity"
)

// maxRetries: un solo reintento, y solo en lecturas. Un comando NUNCA se reintenta: quien reintenta es el
// cliente, con la misma Idempotency-Key, que es lo único que garantiza no duplicar una factura.
const maxRetries = 1

// retryBackoff es corto a propósito: el presupuesto de la petición manda y el usuario está esperando.
const retryBackoff = 50 * time.Millisecond

// Client habla con una API. Uno por servicio: cada uno con su timeout y su pool de conexiones.
type Client struct {
	Service routes.Service
	// BaseURL vacía = el servicio no está desplegado en este ambiente.
	BaseURL string
	HTTP    *http.Client
	Log     *slog.Logger
}

// New arma el cliente de un servicio. timeout cubre la petición completa, incluida la lectura del cuerpo.
func New(service routes.Service, baseURL string, timeout time.Duration, log *slog.Logger) *Client {
	return &Client{
		Service: service,
		BaseURL: strings.TrimRight(baseURL, "/"),
		Log:     log,
		HTTP: &http.Client{
			Timeout: timeout,
			// otelhttp instrumenta la llamada y, con un span activo, inyecta traceparent/tracestate:
			// la traza sigue del portal al gateway y de ahí a la API (arquitectura 8.2).
			Transport: otelhttp.NewTransport(&http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			}),
		},
	}
}

func (c *Client) Configured() bool { return c != nil && c.BaseURL != "" }

// Request es lo que se le manda a una API. Header ya viene filtrado por la lista blanca: este paquete no
// decide qué se propaga, solo lo manda.
type Request struct {
	Method string
	Path   string
	Query  url.Values
	Header http.Header
	Body   io.Reader
	// Retryable permite reintentar. Solo las lecturas lo ponen: el cuerpo de un comando no se puede rebobinar
	// y, sobre todo, reintentarlo podría duplicar una operación.
	Retryable bool
}

// Send ejecuta la petición contra la API. Devuelve la respuesta SIN leer: quien llama cierra el cuerpo.
func (c *Client) Send(ctx context.Context, r Request) (*http.Response, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("%w: %s", app.ErrNotConfigured, c.Service)
	}
	// El token del usuario, tal como llegó. Si no hay, se falla: nunca se sustituye por uno de servicio.
	token, ok := identity.Token(ctx)
	if !ok {
		return nil, fmt.Errorf("%w: %s", app.ErrUnauthenticated, c.Service)
	}

	target := c.BaseURL + r.Path
	if q := r.Query.Encode(); q != "" {
		target += "?" + q
	}

	var lastErr error
	for attempt := 0; attempt <= retriesFor(r); attempt++ {
		if attempt > 0 {
			if err := sleep(ctx, retryBackoff); err != nil {
				return nil, classifyTransport(err)
			}
			c.Log.DebugContext(ctx, "reintentando lectura",
				slog.String("upstream", string(c.Service)), slog.String("path", r.Path))
		}

		req, err := http.NewRequestWithContext(ctx, r.Method, target, r.Body)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", app.ErrUnavailable, err)
		}
		req.Header = r.Header.Clone()
		if req.Header == nil {
			req.Header = http.Header{}
		}
		req.Header.Set("Authorization", "Bearer "+token)
		// El correlation id se pone aquí y no en cada handler: así ninguna llamada de salida puede quedarse
		// sin él, y las tres APIs de una composición comparten el mismo (arquitectura 8.2).
		if id, ok := correlation.FromContext(ctx); ok {
			req.Header.Set(correlation.Header, id.String())
		}

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = classifyTransport(err)
			continue
		}
		if !retryableStatus(resp.StatusCode) {
			return resp, nil
		}
		// Una API que contesta 502/503/504 no procesó nada: reintentar una lectura es seguro.
		lastErr = fmt.Errorf("%w: %s respondió %d", app.ErrUnavailable, c.Service, resp.StatusCode)
		if attempt == retriesFor(r) {
			return resp, nil // se agotaron los reintentos: que el error de la API llegue tal cual
		}
		drain(resp)
	}
	return nil, lastErr
}

// retriesFor: solo las lecturas marcadas como reintentables.
func retriesFor(r Request) int {
	if r.Retryable {
		return maxRetries
	}
	return 0
}

func retryableStatus(code int) bool {
	return code == http.StatusBadGateway || code == http.StatusServiceUnavailable || code == http.StatusGatewayTimeout
}

// classifyTransport separa "no respondió a tiempo" de "no se pudo conectar": el primero es un 504 y el segundo
// un 502. Un timeout queda marcado con las DOS causas, así quien solo distingue disponibilidad no se entera.
func classifyTransport(err error) error {
	if isTimeout(err) {
		return fmt.Errorf("%w: %w: %w", app.ErrUnavailable, app.ErrTimeout, err)
	}
	return fmt.Errorf("%w: %w", app.ErrUnavailable, err)
}

func isTimeout(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// drain deja la conexión reutilizable antes de descartarla.
func drain(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	_ = resp.Body.Close()
}

// maxReadBody acota lo que se decodifica de una API en una composición.
const maxReadBody = 4 << 20

// getJSON es el camino de las COMPOSICIONES: pide, decodifica y traduce el status a un error de `app`.
// El paso directo no pasa por aquí: ese copia la respuesta sin mirarla.
func (c *Client) getJSON(ctx context.Context, path string, dst any) error {
	resp, err := c.Send(ctx, Request{
		Method:    http.MethodGet,
		Path:      path,
		Header:    http.Header{"Accept": []string{"application/json"}},
		Retryable: true,
	})
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// 401/403 de una parte secundaria también degradan: la vista no puede mentir sobre un dato que no vio.
		// Para la parte principal, el StatusError lleva el problema original para devolverlo sin reescribir.
		return newStatusError(c.Service, resp)
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, maxReadBody)).Decode(dst); err != nil {
		return fmt.Errorf("%w: respuesta de %s ilegible: %w", app.ErrUnavailable, c.Service, err)
	}
	return nil
}
