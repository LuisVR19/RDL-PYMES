package app

import "errors"

var (
	// ErrNotFound: el recurso no existe o no es visible para el actor. Nunca distinguimos ambos casos
	// hacia afuera, para no revelar que un recurso de otra organización existe.
	ErrNotFound = errors.New("recurso no encontrado")
	// ErrConflict: la operación choca con el estado actual (ej. email ya usado por otra identidad).
	ErrConflict = errors.New("conflicto con el estado actual")
	// ErrForbidden: el actor es miembro de la organización pero su rol no concede la acción (403).
	ErrForbidden = errors.New("permisos insuficientes")
	// ErrIdempotencyKeyReused: la misma Idempotency-Key llegó con un cuerpo distinto (422).
	ErrIdempotencyKeyReused = errors.New("la Idempotency-Key ya se usó con otra petición")
)
