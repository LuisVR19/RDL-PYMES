package downstream

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"rdl/portal-gateway/internal/app"
	"rdl/portal-gateway/internal/domain/routes"
)

// maxProblemBody: un Problem Details es corto. Se guarda solo para poder devolverlo sin reescribirlo.
const maxProblemBody = 16 << 10

// StatusError es una API que respondió, pero con un status que no sirve para la vista. Guarda su Problem
// Details original: cuando el que falla es la fuente principal de una composición, el portal recibe el error
// de esa API tal cual, con su mismo `type` y su mismo status, y no uno inventado por el gateway.
type StatusError struct {
	Service routes.Service
	Status  int
	// Problem es el cuerpo `application/problem+json` original, o nil si la API no mandó uno.
	Problem []byte
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s respondió %d", e.Service, e.Status)
}

// Unwrap clasifica el status para los casos de uso: 404 es "no existe" y el resto, "no se pudo consultar".
// Así la degradación de una parte secundaria funciona sin conocer este tipo.
func (e *StatusError) Unwrap() error {
	if e.Status == http.StatusNotFound {
		return app.ErrNotFound
	}
	return app.ErrUnavailable
}

func newStatusError(service routes.Service, resp *http.Response) error {
	e := &StatusError{Service: service, Status: resp.StatusCode}
	if isProblem(resp.Header.Get("Content-Type")) {
		if body, err := io.ReadAll(io.LimitReader(resp.Body, maxProblemBody)); err == nil {
			e.Problem = body
		}
	}
	return e
}

func isProblem(contentType string) bool {
	return strings.HasPrefix(strings.TrimSpace(strings.ToLower(contentType)), "application/problem+json")
}
