package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"rdl/einvoice-api/internal/adapters/http/problem"
	"rdl/einvoice-api/internal/app"
	"rdl/einvoice-api/internal/domain/certificate"
	"rdl/einvoice-api/pkg/tenancy"
)

// Problem types de RDL.Contracts/problems/fiscal.yaml que no son comunes a todos los servicios. Se suman los demás
// (validation, conflict, idempotencia, certificado, Hacienda) cuando haya un endpoint que los use.
var (
	problemNoActiveOrganization = problem.New(http.StatusForbidden, "no-active-organization", "Sin organización activa").
					WithDetail("Seleccione una organización en Platform y refresque la sesión.")
	problemMembershipInactive = problem.New(http.StatusForbidden, "membership-inactive", "Membresía inactiva").
					WithDetail("Su acceso a esta organización ya no está activo.")
	problemValidation     = problem.New(http.StatusUnprocessableEntity, "validation", "Datos inválidos")
	problemProfileMissing = problem.New(http.StatusConflict, "fiscal-profile-missing", "Falta el perfil fiscal").
				WithDetail("Configure primero el perfil de contribuyente (PUT /v1/fiscal-profile).")
	problemCertificateInvalid = problem.New(http.StatusUnprocessableEntity, "certificate-invalid", "Certificado inválido")
	problemCertificateExpired = problem.New(http.StatusUnprocessableEntity, "certificate-expired", "Certificado vencido")
	problemMalformed          = problem.New(http.StatusBadRequest, "malformed-request", "Petición mal formada")
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

// errorResponder es el único lugar donde los errores de aplicación se traducen a HTTP: recurso de otra organización
// → 404, rol → 403, campos → 422. Cualquier error no reconocido es 500 sin detalle; la causa real queda solo en el log.
type errorResponder struct{ log *slog.Logger }

func (e errorResponder) write(w http.ResponseWriter, r *http.Request, err error) {
	var verr validationError
	var appVerr app.ValidationError
	var appVerrs app.ValidationErrors
	switch {
	case errors.As(err, &verr):
		problem.Write(w, r, problemValidation.WithErrors(verr.fields...))
	case errors.As(err, &appVerrs):
		fe := make([]problem.FieldError, len(appVerrs))
		for i, v := range appVerrs {
			fe[i] = problem.FieldError{Field: v.Field, Message: v.Message}
		}
		problem.Write(w, r, problemValidation.WithErrors(fe...))
	case errors.As(err, &appVerr):
		problem.Write(w, r, problemValidation.WithErrors(problem.FieldError{Field: appVerr.Field, Message: appVerr.Message}))
	case errors.Is(err, errMalformed):
		problem.Write(w, r, problemMalformed.WithDetail(strings.TrimPrefix(err.Error(), errMalformed.Error()+": ")))
	case errors.Is(err, app.ErrProfileMissing):
		problem.Write(w, r, problemProfileMissing)
	// El detalle del certificado es el de dominio (llave, vigencia, no se pudo abrir): nunca el PIN ni el archivo.
	case errors.Is(err, app.ErrCertificateExpired):
		problem.Write(w, r, problemCertificateExpired.WithDetail(certificateMessage(err)))
	case errors.Is(err, app.ErrCertificateInvalid):
		problem.Write(w, r, problemCertificateInvalid.WithDetail(certificateMessage(err)))
	case errors.Is(err, app.ErrSecretsUnavailable):
		e.log.ErrorContext(r.Context(), "almacén de secretos no disponible", slog.Any("error", err))
		problem.Write(w, r, problem.Unavailable)
	case errors.Is(err, app.ErrForbidden):
		problem.Write(w, r, problem.Forbidden)
	case errors.Is(err, app.ErrNotFound):
		problem.Write(w, r, problem.NotFound)
	default:
		e.log.ErrorContext(r.Context(), "error no controlado", slog.Any("error", err))
		problem.Write(w, r, problem.Internal)
	}
}

// certificateMessage es el primer error de dominio del certificado de la cadena, sin contexto técnico.
func certificateMessage(err error) string {
	for _, target := range []error{certificate.ErrUnreadable, certificate.ErrKeyNotAllowed, certificate.ErrExpired, certificate.ErrNotYetValid} {
		if errors.Is(err, target) {
			return target.Error()
		}
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
