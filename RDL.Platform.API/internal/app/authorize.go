package app

import (
	"rdl/platform-api/internal/domain/membership"
	"rdl/platform-api/pkg/tenancy"
)

// authorize consulta la matriz de permisos del dominio con los roles revalidados del TenantContext.
func authorize(t tenancy.Context, p membership.Permission) error {
	roles := make([]membership.Role, 0, len(t.Roles()))
	for _, r := range t.Roles() {
		roles = append(roles, membership.Role(r))
	}
	if !membership.Can(roles, p) {
		return ErrForbidden
	}
	return nil
}
