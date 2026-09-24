package app

import (
	"errors"

	"rdl/billing-api/internal/domain/permission"
	"rdl/billing-api/pkg/tenancy"
)

var (
	// ErrNotFound: el recurso no existe o no es visible para el actor. Nunca distinguimos ambos casos
	// hacia afuera, para no revelar que un recurso de otra organización existe.
	ErrNotFound = errors.New("recurso no encontrado")
	// ErrForbidden: el actor es miembro de la organización pero su rol no concede la acción (403).
	ErrForbidden = errors.New("permisos insuficientes")
	// ErrIdempotencyKeyReused: la misma Idempotency-Key llegó con un cuerpo distinto (422).
	ErrIdempotencyKeyReused = errors.New("la Idempotency-Key ya se usó con otra petición")
	// ErrCustomerIdentificationTaken: otro cliente de la organización tiene el mismo tipo y número (409).
	ErrCustomerIdentificationTaken = errors.New("identificación de cliente ya registrada")
	// ErrProductCodeTaken: otro producto de la organización tiene el mismo código (409).
	ErrProductCodeTaken = errors.New("código de producto ya usado")
	// ErrCustomerInactive: el documento es para un cliente desactivado (422 customer-inactive).
	ErrCustomerInactive = errors.New("el cliente está desactivado")
)

// authorize consulta la matriz de permisos del dominio con los roles revalidados del TenantContext.
func authorize(t tenancy.Context, p permission.Permission) error {
	roles := make([]permission.Role, 0, len(t.Roles()))
	for _, r := range t.Roles() {
		roles = append(roles, permission.Role(r))
	}
	if !permission.Can(roles, p) {
		return ErrForbidden
	}
	return nil
}
