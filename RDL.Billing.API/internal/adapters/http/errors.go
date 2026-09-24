package http

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"rdl/billing-api/internal/adapters/http/problem"
	"rdl/billing-api/internal/app"
	"rdl/billing-api/internal/domain/customer"
	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/numbering"
	"rdl/billing-api/internal/domain/product"
	"rdl/billing-api/pkg/tenancy"
)

// Tipos de problems/billing.yaml del repo de contratos. Un type que no está registrado allí no se usa.
var (
	problemNoActiveOrganization = problem.New(http.StatusForbidden, "no-active-organization", "Sin organización activa").
					WithDetail("Seleccione una organización activa en Platform y refresque la sesión.")
	problemMembershipInactive = problem.New(http.StatusForbidden, "membership-inactive", "Membresía inactiva").
					WithDetail("Su acceso a esta organización ya no está activo.")
	problemMalformed           = problem.New(http.StatusBadRequest, "malformed-request", "Petición mal formada")
	problemValidation          = problem.New(http.StatusUnprocessableEntity, "validation", "Datos inválidos")
	problemIdempotencyRequired = problem.New(http.StatusBadRequest, "idempotency-key-required", "Idempotency-Key requerida").
					WithDetail("Los comandos POST exigen el header Idempotency-Key (1 a 255 caracteres ASCII visibles).")
	problemIdempotencyReused = problem.New(http.StatusUnprocessableEntity, "idempotency-key-reused", "Idempotency-Key reutilizada").
					WithDetail("La clave ya se usó con una petición distinta. Use una clave nueva para una operación nueva.")
	problemIdentificationTaken = problem.New(http.StatusConflict, "customer-identification-taken", "Identificación ya registrada").
					WithDetail("Otro cliente de la organización tiene el mismo tipo y número de identificación.")
	problemInvoiceNotDraft = problem.New(http.StatusConflict, "invoice-not-draft", "El documento no es un borrador").
				WithDetail("Solo un borrador se edita, cambia de líneas o se descarta. Lo emitido se corrige con notas o anulación.")
	problemInvoiceWithoutLines = problem.New(http.StatusUnprocessableEntity, "invoice-without-lines", "Documento sin líneas").
					WithDetail("Agregue al menos una línea antes de emitir.")
	problemCustomerInactive = problem.New(http.StatusUnprocessableEntity, "customer-inactive", "Cliente inactivo").
				WithDetail("Reactive el cliente o elija otro.")
	problemSequenceInUse = problem.New(http.StatusConflict, "sequence-in-use", "Secuencia en uso").
				WithDetail("La secuencia ya asignó números: cambiar su prefijo o número inicial rompería la serie.")
	problemPrefixTaken = problem.New(http.StatusConflict, "conflict", "Conflicto con el estado actual").
				WithDetail("Otra secuencia del mismo tipo de documento usa ese prefijo; sus números chocarían.")
	problemProductCodeTaken = problem.New(http.StatusConflict, "product-code-taken", "Código de producto ya usado").
				WithDetail("Otro producto de la organización tiene el mismo código.")
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

// validationError: parámetros o campos inválidos (422), con el nombre del campo tal como lo ve el cliente.
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
	case errors.Is(err, errMalformed):
		problem.Write(w, r, problemMalformed.WithDetail(strings.TrimPrefix(err.Error(), errMalformed.Error()+": ")))
	case errors.Is(err, customer.ErrInvalid), errors.Is(err, product.ErrInvalid), errors.Is(err, invoice.ErrInvalid),
		errors.Is(err, numbering.ErrInvalid):
		problem.Write(w, r, problemValidation.WithErrors(domainFieldErrors(err)...))
	case errors.Is(err, errIdempotencyKeyMissing):
		problem.Write(w, r, problemIdempotencyRequired)
	case errors.Is(err, app.ErrIdempotencyKeyReused):
		problem.Write(w, r, problemIdempotencyReused)
	case errors.Is(err, app.ErrCustomerIdentificationTaken):
		problem.Write(w, r, problemIdentificationTaken)
	case errors.Is(err, app.ErrProductCodeTaken):
		problem.Write(w, r, problemProductCodeTaken)
	case errors.Is(err, numbering.ErrInUse):
		problem.Write(w, r, problemSequenceInUse)
	case errors.Is(err, numbering.ErrPrefixTaken):
		problem.Write(w, r, problemPrefixTaken)
	case errors.Is(err, invoice.ErrNotDraft):
		problem.Write(w, r, problemInvoiceNotDraft)
	case errors.Is(err, invoice.ErrWithoutLines):
		problem.Write(w, r, problemInvoiceWithoutLines)
	case errors.Is(err, app.ErrCustomerInactive):
		problem.Write(w, r, problemCustomerInactive)
	case errors.Is(err, app.ErrForbidden):
		problem.Write(w, r, problem.Forbidden)
	case errors.Is(err, app.ErrNotFound):
		problem.Write(w, r, problem.NotFound)
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
	var cfe customer.FieldError
	if errors.As(err, &cfe) {
		return []problem.FieldError{{Field: cfe.Field, Message: cfe.Message}}
	}
	var pfe product.FieldError
	if errors.As(err, &pfe) {
		return []problem.FieldError{{Field: pfe.Field, Message: pfe.Message}}
	}
	var ife invoice.FieldError
	if errors.As(err, &ife) {
		return []problem.FieldError{{Field: ife.Field, Message: ife.Message}}
	}
	var nfe numbering.FieldError
	if errors.As(err, &nfe) {
		return []problem.FieldError{{Field: nfe.Field, Message: nfe.Message}}
	}
	return nil
}
