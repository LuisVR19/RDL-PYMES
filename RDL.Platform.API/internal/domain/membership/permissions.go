package membership

// Permission es una acción autorizable dentro de la organización activa.
type Permission string

const (
	PermOrganizationRead   Permission = "organization.read"
	PermOrganizationUpdate Permission = "organization.update"
	PermMembersRead        Permission = "members.read"
	PermMembersManage      Permission = "members.manage"
	PermInvitationsManage  Permission = "invitations.manage"
	PermBranchesRead       Permission = "branches.read"
	PermBranchesManage     Permission = "branches.manage"
)

// matrix es la ÚNICA fuente de verdad de permisos de Platform. Un rol o permiso nuevo se agrega aquí,
// sin tocar handlers ni casos de uso (que solo preguntan Can).
var matrix = map[Permission][]Role{
	PermOrganizationRead:   AllRoles,
	PermOrganizationUpdate: {RoleOwner, RoleAdmin},
	PermMembersRead:        {RoleOwner, RoleAdmin},
	PermMembersManage:      {RoleOwner, RoleAdmin},
	PermInvitationsManage:  {RoleOwner, RoleAdmin},
	PermBranchesRead:       AllRoles,
	PermBranchesManage:     {RoleOwner, RoleAdmin},
}

// AllRoles es el catálogo core.roles.
var AllRoles = []Role{RoleOwner, RoleAdmin, RoleBiller, RoleCollector, RoleAccountant, RoleReadOnly}

// Can indica si alguno de los roles concede el permiso. Un permiso desconocido se deniega.
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

func (r Role) Valid() bool {
	for _, known := range AllRoles {
		if r == known {
			return true
		}
	}
	return false
}
