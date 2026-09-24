package http

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"rdl/platform-api/internal/adapters/http/problem"
	"rdl/platform-api/internal/app"
	"rdl/platform-api/internal/domain/branch"
	"rdl/platform-api/internal/domain/invitation"
	"rdl/platform-api/internal/domain/membership"
	"rdl/platform-api/internal/domain/organization"
	"rdl/platform-api/internal/domain/user"
	"rdl/platform-api/pkg/tenancy"
)

var (
	problemNoActiveOrganization = problem.New(http.StatusForbidden, "no-active-organization", "Sin organización activa").
					WithDetail("Seleccione una organización con PUT /v1/me/active-organization y refresque la sesión.")
	problemMembershipInactive = problem.New(http.StatusForbidden, "membership-inactive", "Membresía inactiva").
					WithDetail("Su acceso a esta organización ya no está activo.")
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

var (
	problemMalformed    = problem.New(http.StatusBadRequest, "malformed-request", "Petición mal formada")
	problemValidation   = problem.New(http.StatusUnprocessableEntity, "validation", "Datos inválidos")
	problemConflict     = problem.New(http.StatusConflict, "conflict", "Conflicto con el estado actual")
	problemEmailMissing = problem.New(http.StatusUnprocessableEntity, "email-required", "Email requerido").
				WithDetail("La cuenta no tiene un email verificado; es obligatorio para usar la plataforma.")
	problemUserDisabled = problem.New(http.StatusForbidden, "user-disabled", "Usuario deshabilitado")
	problemLastOwner    = problem.New(http.StatusConflict, "last-owner", "La organización quedaría sin owner").
				WithDetail("Debe existir al menos un owner activo. Asigne otro owner antes de cambiar o suspender a este.")
	problemOwnerRequired     = problem.New(http.StatusForbidden, "owner-required", "Solo un owner puede gestionar owners")
	problemInvitationExpired = problem.New(http.StatusGone, "invitation-expired", "Invitación vencida").
					WithDetail("Pida a un administrador de la organización una invitación nueva.")
	problemInvitationNotPending = problem.New(http.StatusConflict, "invitation-not-pending", "La invitación ya no está pendiente")
	problemIdempotencyRequired  = problem.New(http.StatusBadRequest, "idempotency-key-required", "Idempotency-Key requerida").
					WithDetail("Los comandos POST exigen el header Idempotency-Key (1 a 255 caracteres ASCII visibles).")
	problemIdempotencyReused = problem.New(http.StatusUnprocessableEntity, "idempotency-key-reused", "Idempotency-Key reutilizada").
					WithDetail("La clave ya se usó con una petición distinta. Use una clave nueva para una operación nueva.")
)

// errorResponder es el único lugar donde los errores de dominio y de aplicación se traducen a HTTP.
// Cualquier error no reconocido es 500 sin detalle; la causa real queda solo en el log.
type errorResponder struct{ log *slog.Logger }

func (e errorResponder) write(w http.ResponseWriter, r *http.Request, err error) {
	var verr validationError
	switch {
	case errors.As(err, &verr):
		problem.Write(w, r, problemValidation.WithErrors(verr.fields...))
	case errors.Is(err, errMalformed):
		problem.Write(w, r, problemMalformed.WithDetail(strings.TrimPrefix(err.Error(), errMalformed.Error()+": ")))
	case errors.Is(err, membership.ErrLastOwner):
		problem.Write(w, r, problemLastOwner)
	case errors.Is(err, membership.ErrOwnerRequired):
		problem.Write(w, r, problemOwnerRequired)
	case errors.Is(err, membership.ErrEmptyChange):
		problem.Write(w, r, problemValidation.WithErrors(problem.FieldError{Field: "role", Message: "indique role o status"}))
	case errors.Is(err, membership.ErrInvalidRole):
		problem.Write(w, r, problemValidation.WithErrors(problem.FieldError{Field: "role", Message: "no es un rol válido"}))
	case errors.Is(err, membership.ErrInvalidStatus):
		problem.Write(w, r, problemValidation.WithErrors(problem.FieldError{Field: "status", Message: "debe ser active o suspended"}))
	case errors.Is(err, invitation.ErrExpired):
		problem.Write(w, r, problemInvitationExpired)
	case errors.Is(err, invitation.ErrNotPending):
		problem.Write(w, r, problemInvitationNotPending)
	case errors.Is(err, invitation.ErrInvalidEmail):
		problem.Write(w, r, problemValidation.WithErrors(problem.FieldError{Field: "email", Message: "debe ser un email válido"}))
	case errors.Is(err, organization.ErrInvalid), errors.Is(err, branch.ErrInvalid):
		problem.Write(w, r, problemValidation.WithErrors(domainFieldErrors(err)...))
	case errors.Is(err, errIdempotencyKeyMissing):
		problem.Write(w, r, problemIdempotencyRequired)
	case errors.Is(err, app.ErrIdempotencyKeyReused):
		problem.Write(w, r, problemIdempotencyReused)
	case errors.Is(err, app.ErrForbidden):
		problem.Write(w, r, problem.Forbidden)
	case errors.Is(err, app.ErrNotFound):
		problem.Write(w, r, problem.NotFound)
	case errors.Is(err, app.ErrConflict):
		// El detalle de un conflicto lo redacta el adapter (nunca es un error de la base) y ayuda a corregir.
		problem.Write(w, r, problemConflict.WithDetail(strings.TrimPrefix(err.Error(), app.ErrConflict.Error()+": ")))
	case errors.Is(err, user.ErrEmailRequired):
		problem.Write(w, r, problemEmailMissing)
	case errors.Is(err, user.ErrDisabled):
		problem.Write(w, r, problemUserDisabled)
	default:
		e.log.ErrorContext(r.Context(), "error no controlado", slog.Any("error", err))
		problem.Write(w, r, problem.Internal)
	}
}

// domainFieldErrors extrae los campos inválidos de un error de validación del dominio (posiblemente varios unidos).
func domainFieldErrors(err error) []problem.FieldError {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		var out []problem.FieldError
		for _, e := range joined.Unwrap() {
			out = append(out, domainFieldErrors(e)...)
		}
		return out
	}
	var ofe organization.FieldError
	if errors.As(err, &ofe) {
		return []problem.FieldError{{Field: ofe.Field, Message: ofe.Message}}
	}
	var bfe branch.FieldError
	if errors.As(err, &bfe) {
		return []problem.FieldError{{Field: bfe.Field, Message: bfe.Message}}
	}
	return nil
}
