// Package permission es la matriz de permisos de Billing: la ÚNICA fuente de verdad de quién puede hacer qué.
// Los roles salen de la membresía revalidada en core (no del claim org_roles). Un rol o permiso nuevo se agrega
// aquí, sin tocar handlers ni casos de uso, que solo preguntan Can.
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

// AllRoles es el catálogo core.roles.
var AllRoles = []Role{RoleOwner, RoleAdmin, RoleBiller, RoleCollector, RoleAccountant, RoleReadOnly}

// Permission es una acción autorizable dentro de la organización activa.
type Permission string

const (
	CustomersRead   Permission = "customers.read"
	CustomersManage Permission = "customers.manage"
	ProductsRead    Permission = "products.read"
	ProductsManage  Permission = "products.manage"
	InvoicesRead    Permission = "invoices.read"
	InvoicesManage  Permission = "invoices.manage" // crear, editar, reemplazar líneas y descartar borradores
	InvoicesIssue   Permission = "invoices.issue"
	SequencesRead   Permission = "sequences.read"
	SequencesManage Permission = "sequences.manage"
)

// writers operan la facturación. Propuesta del prompt P4 para revisar en equipo: collector, accountant y
// read_only solo leen.
var writers = []Role{RoleOwner, RoleAdmin, RoleBiller}

var matrix = map[Permission][]Role{
	CustomersRead:   AllRoles,
	CustomersManage: writers,
	ProductsRead:    AllRoles,
	ProductsManage:  writers,
	InvoicesRead:    AllRoles,
	InvoicesManage:  writers,
	InvoicesIssue:   writers,
	SequencesRead:   {RoleOwner, RoleAdmin},
	SequencesManage: {RoleOwner, RoleAdmin},
}

// Can indica si alguno de los roles concede el permiso. Un permiso o rol desconocido se deniega.
func Can(roles []Role, p Permission) bool {
	for _, allowed := range matrix[p] {
		for _, r := range roles {
			if r == allowed {
				return true
			}
		}
	}
	return false
}
