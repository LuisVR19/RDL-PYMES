// Package permission es la ÚNICA fuente de verdad de la matriz de permisos de Receivables (docs/PLAN.md §4).
// Handlers y casos de uso solo preguntan Can; un rol o permiso nuevo se agrega aquí.
package permission

// Role es un código del catálogo core.roles.
type Role string

const (
	RoleOwner      Role = "owner"
	RoleAdmin      Role = "admin"
	RoleBiller     Role = "biller"
	RoleCollector  Role = "collector"
	RoleAccountant Role = "accountant"
	RoleReadOnly   Role = "read_only"
)

// Permission es una acción autorizable dentro de la organización activa.
type Permission string

const (
	// ReceivablesRead: cuentas, pagos, aging y seguimientos.
	ReceivablesRead Permission = "receivables.read"
	// InvoiceBalanceRead: saldo por factura para el BFF (R10: incluye biller).
	InvoiceBalanceRead Permission = "receivables.invoice_balance.read"
	// PaymentsWrite: registrar y aplicar pagos, seguimientos y promesas.
	PaymentsWrite Permission = "receivables.payments.write"
	// PaymentsReverse: anular pagos y revertir aplicaciones.
	PaymentsReverse Permission = "receivables.payments.reverse"
)

var readers = []Role{RoleOwner, RoleAdmin, RoleCollector, RoleAccountant, RoleReadOnly}

var matrix = map[Permission][]Role{
	ReceivablesRead:    readers,
	InvoiceBalanceRead: append(readers[:len(readers):len(readers)], RoleBiller),
	PaymentsWrite:      {RoleOwner, RoleAdmin, RoleCollector},
	PaymentsReverse:    {RoleOwner, RoleAdmin},
}

// Can indica si alguno de los roles concede el permiso. Un permiso o rol desconocido se deniega.
func Can(roles []string, p Permission) bool {
	for _, allowed := range matrix[p] {
		for _, r := range roles {
			if Role(r) == allowed {
				return true
			}
		}
	}
	return false
}
