// Package permission es la ÚNICA fuente de verdad de la matriz de permisos de fiscal. Sale de los roles que
// RDL.Contracts/openapi/fiscal.yaml declara en cada operación. Handlers y casos de uso solo preguntan Can; un rol o
// permiso nuevo se agrega aquí.
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
	// FiscalProfileRead: getFiscalProfile (owner, admin, accountant).
	FiscalProfileRead Permission = "fiscal.profile.read"
	// FiscalProfileWrite: putFiscalProfile (owner, admin).
	FiscalProfileWrite Permission = "fiscal.profile.write"
	// CertificateUpload: uploadCertificate (owner).
	CertificateUpload Permission = "fiscal.certificate.upload"
	// HaciendaCredentialsWrite: usuario y contraseña del API de Hacienda (owner, como el certificado). Ruta propia de
	// este repo, a proponer en RDL.Contracts/openapi/fiscal.yaml.
	HaciendaCredentialsWrite Permission = "fiscal.hacienda_credentials.write" // #nosec G101 -- nombre de un permiso, no una credencial
	// EstablishmentsRead: listEstablishments (owner, admin, accountant).
	EstablishmentsRead Permission = "fiscal.establishments.read"
	// EstablishmentsWrite: createEstablishment (owner, admin).
	EstablishmentsWrite Permission = "fiscal.establishments.write"
	// DocumentsRead: documentos electrónicos y sus archivos (todos los roles).
	DocumentsRead Permission = "fiscal.documents.read"
	// DocumentsRetry: retryElectronicDocument (owner, admin).
	DocumentsRetry Permission = "fiscal.documents.retry"
	// CatalogsRead: CABYS y catálogos para formularios (todos los roles).
	CatalogsRead Permission = "fiscal.catalogs.read"
)

var everyone = []Role{RoleOwner, RoleAdmin, RoleBiller, RoleCollector, RoleAccountant, RoleReadOnly}

var matrix = map[Permission][]Role{
	FiscalProfileRead:        {RoleOwner, RoleAdmin, RoleAccountant},
	FiscalProfileWrite:       {RoleOwner, RoleAdmin},
	CertificateUpload:        {RoleOwner},
	HaciendaCredentialsWrite: {RoleOwner},
	EstablishmentsRead:       {RoleOwner, RoleAdmin, RoleAccountant},
	EstablishmentsWrite:      {RoleOwner, RoleAdmin},
	DocumentsRead:            everyone,
	DocumentsRetry:           {RoleOwner, RoleAdmin},
	CatalogsRead:             everyone,
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
