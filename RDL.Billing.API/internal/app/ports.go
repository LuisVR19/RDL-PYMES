// Package app contiene los casos de uso (un struct por caso) y los puertos que necesitan.
// Los puertos se definen aquí, del lado del consumidor; los adapters los implementan.
package app

import (
	"context"
	"time"

	"github.com/google/uuid"

	"rdl/billing-api/internal/domain/customer"
	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/money"
	"rdl/billing-api/internal/domain/numbering"
	"rdl/billing-api/internal/domain/product"
	"rdl/billing-api/pkg/tenancy"
)

// TxManager es la unidad de trabajo: cada función corre en una transacción con la sesión RLS ya fijada,
// y todo lo que escribe (entidad, historial, audit, outbox) se confirma o se descarta junto.
type TxManager interface {
	WithinTenantTx(ctx context.Context, t tenancy.Context, fn func(ctx context.Context, tx Tx) error) error
}

// Tx expone los repositorios ligados a la transacción en curso.
type Tx interface {
	Customers() CustomerRepository
	Products() ProductRepository
	Invoices() InvoiceRepository
	Catalog() Catalog
	Sequences() SequenceRepository
	Outbox() EventOutbox
	Idempotency() IdempotencyStore
	Audit() AuditRecorder
}

type CustomerRepository interface {
	// Create devuelve ErrCustomerIdentificationTaken si otro cliente de la organización tiene la misma identificación.
	Create(ctx context.Context, c customer.Customer) (customer.Customer, error)
	List(ctx context.Context, organizationID uuid.UUID, q CustomerQuery) ([]customer.Customer, error)
	// Get y GetForUpdate devuelven ErrNotFound si el cliente no existe o es de otra organización.
	Get(ctx context.Context, organizationID, id uuid.UUID) (customer.Customer, error)
	GetForUpdate(ctx context.Context, organizationID, id uuid.UUID) (customer.Customer, error)
	Update(ctx context.Context, c customer.Customer) (customer.Customer, error)
}

type CustomerQuery struct {
	Search string // nombre legal, comercial o número de identificación; vacío = sin filtro
	Active *bool  // nil = todos
	After  *PageCursor
	Limit  int
}

type ProductRepository interface {
	// Create y Update devuelven ErrProductCodeTaken si otro producto de la organización tiene el mismo código.
	// Guardan los impuestos del producto junto con él.
	Create(ctx context.Context, p product.Product) (product.Product, error)
	List(ctx context.Context, organizationID uuid.UUID, q ProductQuery) ([]product.Product, error)
	// Get y GetForUpdate devuelven ErrNotFound si el producto no existe o es de otra organización.
	Get(ctx context.Context, organizationID, id uuid.UUID) (product.Product, error)
	GetForUpdate(ctx context.Context, organizationID, id uuid.UUID) (product.Product, error)
	Update(ctx context.Context, p product.Product) (product.Product, error)
}

type ProductQuery struct {
	Search string // código o descripción; vacío = sin filtro
	Active *bool  // nil = todos
	After  *PageCursor
	Limit  int
}

type InvoiceRepository interface {
	// Create guarda el encabezado, las líneas y sus impuestos; devuelve el documento con id y fechas.
	Create(ctx context.Context, inv invoice.Invoice) (invoice.Invoice, error)
	// Get y GetForUpdate devuelven ErrNotFound si no existe o es de otra organización. Traen las líneas.
	Get(ctx context.Context, organizationID, id uuid.UUID) (invoice.Invoice, error)
	GetForUpdate(ctx context.Context, organizationID, id uuid.UUID) (invoice.Invoice, error)
	// SaveDraft reescribe encabezado, totales y líneas de un borrador. ErrNotDraft si ya no lo es.
	SaveDraft(ctx context.Context, inv invoice.Invoice) (invoice.Invoice, error)
	// DeleteDraft borra el borrador y sus líneas. ErrNotDraft si ya no lo es.
	DeleteDraft(ctx context.Context, organizationID, id uuid.UUID) error
	// MarkIssued guarda la transición draft → issued (número, fecha, vencimiento y snapshot del cliente).
	// ErrNotDraft si el documento ya no es un borrador.
	MarkIssued(ctx context.Context, inv invoice.Invoice) (invoice.Invoice, error)
	List(ctx context.Context, organizationID uuid.UUID, q InvoiceQuery) ([]invoice.Invoice, error)
	History(ctx context.Context, organizationID, id uuid.UUID) ([]StatusChange, error)
	AddStatusChange(ctx context.Context, organizationID, id uuid.UUID, c StatusChange) error
}

// EventOutbox escribe eventos en integration.outbox_messages en la transacción en curso. Arma el evento del
// contrato, lo valida contra su JSON Schema y solo entonces lo escribe; el correlationId sale del contexto.
// Publicarlo es trabajo del worker de infraestructura (P2), no de esta API.
type EventOutbox interface {
	InvoiceIssued(ctx context.Context, inv invoice.Invoice, issueDate string) error
}

type InvoiceQuery struct {
	DocumentType       invoice.DocumentType // vacío = todos
	Status             invoice.Status       // vacío = todos
	CustomerID         *uuid.UUID
	RequiresCorrection *bool
	IssuedFrom         string // YYYY-MM-DD en la zona de la organización; vacío = sin límite
	IssuedTo           string
	Timezone           string // zona IANA de la organización, para las fechas de emisión
	After              *PageCursor
	Limit              int
}

type StatusChange struct {
	From            invoice.Status // vacío en el primer registro
	To              invoice.Status
	Reason          string
	ChangedByUserID *uuid.UUID
	ChangedAt       time.Time
}

type SequenceRepository interface {
	// Lock serializa, hasta el fin de la transacción, todo lo que toca las secuencias de un tipo de documento en la
	// organización (configurar y asignar números).
	Lock(ctx context.Context, organizationID uuid.UUID, documentType string) error
	List(ctx context.Context, organizationID uuid.UUID) ([]numbering.Sequence, error)
	// GetForUpdate bloquea la secuencia del alcance; found = false si no existe.
	GetForUpdate(ctx context.Context, organizationID uuid.UUID, scope numbering.Scope) (s numbering.Sequence, found bool, err error)
	// Save inserta (ID nulo) o actualiza.
	Save(ctx context.Context, s numbering.Sequence) (numbering.Sequence, error)
}

// Catalog lee lo que Billing necesita de otros schemas, siempre bajo la sesión de la organización activa.
type Catalog interface {
	// Organization devuelve la moneda local y la zona horaria de la organización activa.
	Organization(ctx context.Context, organizationID uuid.UUID) (OrganizationSettings, error)
	// Branch devuelve ErrNotFound si la sucursal no existe o es de otra organización.
	Branch(ctx context.Context, organizationID, id uuid.UUID) (Branch, error)
	// TaxRates devuelve la tasa vigente de cada código del catálogo fiscal.tax_rates. Los códigos que no están (o no
	// están activos) no aparecen en el mapa.
	TaxRates(ctx context.Context, codes []string) (map[string]money.Percentage, error)
}

type OrganizationSettings struct {
	LocalCurrency money.Currency
	Timezone      string
}

type Branch struct {
	ID       uuid.UUID
	IsActive bool
}

// PageCursor es la posición del último elemento de una página: (instante de orden, id) para desempatar.
type PageCursor struct {
	At time.Time
	ID uuid.UUID
}

// Paginación por cursor (convenciones §9).
const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// IdempotencyStore persiste las claves Idempotency-Key en integration.idempotency_keys (service = 'billing'), en la
// misma transacción que el comando: si el comando falla, la reserva desaparece con él.
type IdempotencyStore interface {
	// Claim reserva la clave. Si ya existía (y no venció) devuelve el registro previo en lugar de reservar.
	// Una petición concurrente con la misma clave espera a que la primera confirme y luego ve su registro.
	Claim(ctx context.Context, organizationID uuid.UUID, key, requestHash string, ttl time.Duration) (*IdempotencyRecord, error)
	Complete(ctx context.Context, organizationID uuid.UUID, key string, result IdempotencyRecord) error
}

// IdempotencyRecord guarda la referencia al resultado, no la respuesta HTTP: al repetir, el caso de uso relee el
// recurso (igual que Platform). Si el recurso cambió entre ambos POST, el reintento ve su estado actual.
type IdempotencyRecord struct {
	RequestHash string
	Status      int
	Result      map[string]string
}

// AuditRecorder escribe en audit.audit_events dentro de la transacción del cambio. El correlationId, la IP y
// el user-agent los toma el adapter del contexto del request.
type AuditRecorder interface {
	Record(ctx context.Context, e AuditEvent) error
}

type AuditEvent struct {
	OrganizationID uuid.UUID
	ActorUserID    uuid.UUID
	Action         string
	EntityType     string
	EntityID       uuid.UUID
	Before         any
	After          any
	Reason         string
}
