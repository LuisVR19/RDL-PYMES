package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"rdl/receivables-api/internal/adapters/http/problem"
	"rdl/receivables-api/internal/app"
	"rdl/receivables-api/internal/domain/amount"
	"rdl/receivables-api/internal/domain/collection"
	"rdl/receivables-api/internal/domain/payment"
	"rdl/receivables-api/internal/domain/receivable"
	"rdl/receivables-api/internal/domain/settlement"
	"rdl/receivables-api/pkg/tenancy"
)

// Problem types de RDL.Contracts/problems/receivables.yaml. customer-mismatch (R11) todavía no está en el catálogo:
// TODO(contratos) PR con el tipo nuevo.
var (
	problemNoActiveOrganization = problem.New(http.StatusForbidden, "no-active-organization", "Sin organización activa").
					WithDetail("Seleccione una organización en Platform y refresque la sesión.")
	problemMembershipInactive = problem.New(http.StatusForbidden, "membership-inactive", "Membresía inactiva").
					WithDetail("Su acceso a esta organización ya no está activo.")
	problemValidation  = problem.New(http.StatusUnprocessableEntity, "validation", "Datos inválidos")
	problemMalformed   = problem.New(http.StatusBadRequest, "malformed-request", "Petición mal formada")
	problemConflict    = problem.New(http.StatusConflict, "conflict", "Conflicto con el estado actual")
	problemIdemMissing = problem.New(http.StatusBadRequest, "idempotency-key-required", "Idempotency-Key requerida").
				WithDetail("Los comandos POST exigen el header Idempotency-Key (1 a 255 caracteres ASCII visibles).")
	problemIdemReused = problem.New(http.StatusUnprocessableEntity, "idempotency-key-reused", "Idempotency-Key reutilizada").
				WithDetail("La clave ya se usó con una petición distinta. Use una clave nueva para una operación nueva.")
	problemExceedsPayment = problem.New(http.StatusUnprocessableEntity, "application-exceeds-payment", "La aplicación supera el pago").
				WithDetail("La suma aplicada superaría el monto disponible del pago.")
	problemExceedsBalance = problem.New(http.StatusUnprocessableEntity, "application-exceeds-balance", "La aplicación supera el saldo").
				WithDetail("El monto aplicado es mayor que el saldo de la cuenta.")
	problemCurrencyMismatch = problem.New(http.StatusUnprocessableEntity, "currency-mismatch", "Moneda distinta").
				WithDetail("En V1 un pago solo se aplica a cuentas en su misma moneda.")
	problemCustomerMismatch = problem.New(http.StatusUnprocessableEntity, "customer-mismatch", "Cliente distinto").
				WithDetail("El pago y la cuenta pertenecen a clientes distintos.")
	problemPaymentVoided = problem.New(http.StatusConflict, "payment-voided", "Pago anulado").
				WithDetail("El pago está anulado: no se aplica ni se vuelve a anular.")
	problemApplicationReversed = problem.New(http.StatusConflict, "application-reversed", "Aplicación revertida").
					WithDetail("La aplicación ya fue revertida.")
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

// errorResponder es el único lugar donde los errores de dominio y de aplicación se traducen a HTTP: recurso de otra
// organización → 404, rol → 403, estado → 409, regla de negocio → 422. Cualquier error no reconocido es 500 sin
// detalle; la causa real queda solo en el log.
type errorResponder struct{ log *slog.Logger }

func (e errorResponder) write(w http.ResponseWriter, r *http.Request, err error) {
	var verr validationError
	var appVerr app.ValidationError
	switch {
	case errors.As(err, &verr):
		problem.Write(w, r, problemValidation.WithErrors(verr.fields...))
	case errors.As(err, &appVerr):
		problem.Write(w, r, problemValidation.WithErrors(problem.FieldError{Field: appVerr.Field, Message: appVerr.Message}))
	case errors.Is(err, errMalformed):
		problem.Write(w, r, problemMalformed.WithDetail(strings.TrimPrefix(err.Error(), errMalformed.Error()+": ")))
	case errors.Is(err, errIdempotencyKeyMissing):
		problem.Write(w, r, problemIdemMissing)
	case errors.Is(err, app.ErrIdempotencyKeyReused):
		problem.Write(w, r, problemIdemReused)
	case errors.Is(err, app.ErrForbidden):
		problem.Write(w, r, problem.Forbidden)
	case errors.Is(err, app.ErrNotFound):
		problem.Write(w, r, problem.NotFound)

	case errors.Is(err, payment.ErrExceedsPayment):
		problem.Write(w, r, problemExceedsPayment)
	case errors.Is(err, receivable.ErrExceedsBalance):
		problem.Write(w, r, problemExceedsBalance)
	case errors.Is(err, settlement.ErrCurrencyMismatch):
		problem.Write(w, r, problemCurrencyMismatch)
	case errors.Is(err, settlement.ErrCustomerMismatch):
		problem.Write(w, r, problemCustomerMismatch)
	case errors.Is(err, payment.ErrVoided):
		problem.Write(w, r, problemPaymentVoided)
	case errors.Is(err, payment.ErrApplicationReversed), errors.Is(err, receivable.ErrApplicationReversed):
		problem.Write(w, r, problemApplicationReversed)
	case errors.Is(err, amount.ErrInvalid):
		problem.Write(w, r, problemValidation.WithErrors(problem.FieldError{Field: "amount", Message: err.Error()}))
	case errors.Is(err, collection.ErrPromiseExceedsBalance):
		problem.Write(w, r, problemValidation.WithErrors(problem.FieldError{Field: "promisedAmount", Message: err.Error()}))
	case errors.Is(err, collection.ErrPromiseInPast):
		problem.Write(w, r, problemValidation.WithErrors(problem.FieldError{Field: "promisedOn", Message: err.Error()}))
	case errors.Is(err, collection.ErrInvalidType), errors.Is(err, collection.ErrNotesRequired),
		errors.Is(err, collection.ErrInvalidPromiseStatus):
		problem.Write(w, r, problemValidation.WithDetail(err.Error()))

	// Estado que no permite la operación: el mensaje del dominio es para el usuario (no trae datos internos).
	case errors.Is(err, receivable.ErrCancelled), errors.Is(err, receivable.ErrDuplicateApplication),
		errors.Is(err, payment.ErrDuplicateApplication), errors.Is(err, collection.ErrNotCollectible),
		errors.Is(err, collection.ErrPromiseClosed):
		problem.Write(w, r, problemConflict.WithDetail(domainMessage(err)))
	default:
		e.log.ErrorContext(r.Context(), "error no controlado", slog.Any("error", err))
		problem.Write(w, r, problem.Internal)
	}
}

// domainMessage es el primer error de dominio de la cadena, sin el contexto técnico que se le haya agregado.
func domainMessage(err error) string {
	for _, target := range []error{
		receivable.ErrCancelled, receivable.ErrDuplicateApplication, payment.ErrDuplicateApplication,
		collection.ErrNotCollectible, collection.ErrPromiseClosed,
	} {
		if errors.Is(err, target) {
			return target.Error()
		}
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
