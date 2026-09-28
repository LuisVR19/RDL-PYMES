// Package collection modela la gestión de cobro: seguimientos (llamadas, correos, visitas...) y promesas de pago.
// Son historial auditable: se crean y una promesa solo cambia de estado (R8), nunca se editan ni se borran.
package collection

import (
	"errors"

	"github.com/shopspring/decimal"

	"rdl/receivables-api/internal/domain/amount"
	"rdl/receivables-api/internal/domain/civil"
	"rdl/receivables-api/internal/domain/receivable"
)

var (
	// ErrInvalidType: tipo de seguimiento fuera del catálogo (collection_followups_type_ck).
	ErrInvalidType = errors.New("tipo de seguimiento inválido")
	// ErrNotesRequired: un seguimiento sin notas no deja constancia de nada.
	ErrNotesRequired = errors.New("las notas del seguimiento son obligatorias")
	// ErrNotCollectible: la cuenta está pagada o anulada; no admite promesas (409).
	ErrNotCollectible = errors.New("la cuenta no tiene saldo por cobrar")
	// ErrPromiseExceedsBalance: se promete más de lo que se debe (422).
	ErrPromiseExceedsBalance = errors.New("la promesa supera el saldo de la cuenta")
	// ErrPromiseInPast: la fecha prometida es anterior al día de negocio de hoy (422).
	ErrPromiseInPast = errors.New("la fecha prometida ya pasó")
	// ErrPromiseClosed: la promesa ya se cumplió, se incumplió o se canceló; los tres estados son finales (409).
	ErrPromiseClosed = errors.New("la promesa ya está cerrada")
	// ErrInvalidPromiseStatus: destino que no existe o que no es un cierre.
	ErrInvalidPromiseStatus = errors.New("estado de promesa inválido")
)

type FollowUpType string

const (
	FollowUpCall    FollowUpType = "call"
	FollowUpEmail   FollowUpType = "email"
	FollowUpVisit   FollowUpType = "visit"
	FollowUpMessage FollowUpType = "message"
	FollowUpNote    FollowUpType = "note"
)

func (t FollowUpType) Valid() bool {
	switch t {
	case FollowUpCall, FollowUpEmail, FollowUpVisit, FollowUpMessage, FollowUpNote:
		return true
	}
	return false
}

// ValidateFollowUp revisa un seguimiento nuevo. Se admite sobre cualquier cuenta: también se documenta la gestión
// de una cuenta ya pagada o anulada.
func ValidateFollowUp(t FollowUpType, notes string) error {
	if !t.Valid() {
		return ErrInvalidType
	}
	if notes == "" {
		return ErrNotesRequired
	}
	return nil
}

// PromiseStatus sigue la decisión R8: pending → kept | broken | cancelled, los tres finales.
type PromiseStatus string

const (
	PromisePending   PromiseStatus = "pending"
	PromiseKept      PromiseStatus = "kept"
	PromiseBroken    PromiseStatus = "broken"
	PromiseCancelled PromiseStatus = "cancelled"
)

// ValidatePromise revisa una promesa nueva contra la cuenta: debe estar cobrable, el monto cabe en el saldo y la
// fecha no es anterior a hoy (fecha de negocio de la organización).
func ValidatePromise(status receivable.Status, balance, promised decimal.Decimal, promisedOn, today civil.Date) error {
	if !status.Collectible() {
		return ErrNotCollectible
	}
	if err := amount.Positive(promised); err != nil {
		return err
	}
	if promised.GreaterThan(balance) {
		return ErrPromiseExceedsBalance
	}
	if promisedOn.Before(today) {
		return ErrPromiseInPast
	}
	return nil
}

// Transition valida el cierre de una promesa. "Cumplida" la marca el usuario: en V1 no hay automatismo (R8).
func Transition(from, to PromiseStatus) error {
	switch to {
	case PromiseKept, PromiseBroken, PromiseCancelled:
	default:
		return ErrInvalidPromiseStatus
	}
	if from != PromisePending {
		return ErrPromiseClosed
	}
	return nil
}
