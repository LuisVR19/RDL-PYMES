package receivable

import "errors"

var (
	// ErrNegativeBalance: la operación dejaría el saldo por debajo de cero (la base también lo rechaza).
	ErrNegativeBalance = errors.New("el saldo de la cuenta quedaría negativo")
	// ErrExceedsBalance: la aplicación es mayor que el saldo (application-exceeds-balance, 422).
	ErrExceedsBalance = errors.New("la aplicación supera el saldo de la cuenta")
	// ErrCancelled: la cuenta está anulada; no admite aplicaciones ni ajustes (409; en un evento, dead letter, R4).
	ErrCancelled = errors.New("la cuenta por cobrar está anulada")
	// ErrDuplicateApplication: ya hay una aplicación vigente del mismo pago a esta cuenta (UK parcial de la base, 409).
	ErrDuplicateApplication = errors.New("el pago ya tiene una aplicación vigente a esta cuenta")
	// ErrApplicationNotFound: la aplicación no es de esta cuenta.
	ErrApplicationNotFound = errors.New("la aplicación no existe en esta cuenta")
	// ErrApplicationReversed: la aplicación ya estaba revertida (application-reversed, 409).
	ErrApplicationReversed = errors.New("la aplicación ya fue revertida")
	// ErrReasonRequired: revertir o anular exige un motivo.
	ErrReasonRequired = errors.New("el motivo es obligatorio")
	// ErrCreditExceedsDebt: la nota de crédito supera el total adeudado aun revirtiendo todas las aplicaciones (R3,
	// dead letter).
	ErrCreditExceedsDebt = errors.New("la nota de crédito supera el total adeudado de la cuenta")
	// ErrNothingToCancel: la factura se anula con saldo cero después de revertir las aplicaciones (quedó saldada
	// solo con notas de crédito). La base no puede registrar un ajuste de 0: dead letter y TODO en contratos.
	ErrNothingToCancel = errors.New("la cuenta no tiene saldo que anular")
	// ErrConflictingAdjustment: llegó otra vez el mismo documento de origen con un monto distinto (dead letter).
	ErrConflictingAdjustment = errors.New("el documento de origen ya se registró con otro monto")
	// ErrInvalidDates: el vencimiento es anterior a la emisión (receivables_due_ck).
	ErrInvalidDates = errors.New("el vencimiento es anterior a la emisión")
	// ErrInconsistent: los datos cargados de la base no cumplen las invariantes del agregado.
	ErrInconsistent = errors.New("la cuenta por cobrar cargada no es consistente")
)
