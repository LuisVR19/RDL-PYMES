package app

import (
	"errors"

	"rdl/receivables-api/internal/domain/permission"
	"rdl/receivables-api/pkg/tenancy"
)

var (
	// ErrNotFound: el recurso no existe o es de otra organización. Hacia afuera nunca se distinguen (404).
	ErrNotFound = errors.New("recurso no encontrado")
	// ErrForbidden: el actor es miembro de la organización pero su rol no concede la acción (403).
	ErrForbidden = errors.New("permisos insuficientes")
)

// authorize consulta la matriz de permisos con los roles revalidados del TenantContext.
func authorize(t tenancy.Context, p permission.Permission) error {
	if !permission.Can(t.Roles(), p) {
		return ErrForbidden
	}
	return nil
}
