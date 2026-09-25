package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"rdl/receivables-api/internal/adapters/http/problem"
	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/pkg/tenancy"
)

var (
	problemNoActiveOrganization = problem.New(http.StatusForbidden, "no-active-organization", "Sin organización activa").
					WithDetail("Seleccione una organización en Platform y refresque la sesión.")
	problemMembershipInactive = problem.New(http.StatusForbidden, "membership-inactive", "Membresía inactiva").
					WithDetail("Su acceso a esta organización ya no está activo.")
	problemValidation = problem.New(http.StatusUnprocessableEntity, "validation", "Datos inválidos")
)

// tenancyErrorWriter es el único lugar donde los errores de autenticación y tenancy se convierten en HTTP.
// El detalle técnico (firma, exp, aud...) va al log, nunca a la respuesta.
func tenancyErrorWriter(log *slog.Logger) tenancy.ErrorWriter {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		switch {
		case errors.Is(err, tenancy.ErrUnauthenticated):
			log.InfoContext(r.Context(), "autenticación rechazada", slog.Any("error", err))
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			problem.Write(w, r, problem.Unauthorized)
		case errors.Is(err, tenancy.ErrNoActiveOrganization):
			problem.Write(w, r, problemNoActiveOrganization)
		case errors.Is(err, tenancy.ErrNoMembership):
			problem.Write(w, r, problemMembershipInactive)
		default:
			log.ErrorContext(r.Context(), "error resolviendo tenancy", slog.Any("error", err))
			problem.Write(w, r, problem.Internal)
		}
	}
}

// validationError agrupa los campos inválidos de una petición (422).
type validationError struct{ fields []problem.FieldError }

func (e validationError) Error() string { return "validación fallida" }

// errorResponder es el único lugar donde los errores de dominio y de aplicación se traducen a HTTP.
// Cualquier error no reconocido es 500 sin detalle; la causa real queda solo en el log.
type errorResponder struct{ log *slog.Logger }

func (e errorResponder) write(w http.ResponseWriter, r *http.Request, err error) {
	var verr validationError
	switch {
	case errors.As(err, &verr):
		problem.Write(w, r, problemValidation.WithErrors(verr.fields...))
	case errors.Is(err, app.ErrForbidden):
		problem.Write(w, r, problem.Forbidden)
	case errors.Is(err, app.ErrNotFound):
		problem.Write(w, r, problem.NotFound)
	default:
		e.log.ErrorContext(r.Context(), "error no controlado", slog.Any("error", err))
		problem.Write(w, r, problem.Internal)
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
