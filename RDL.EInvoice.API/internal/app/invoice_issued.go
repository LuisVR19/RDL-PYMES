package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"rdl/einvoice-api/internal/domain/document"
)

// InvoiceIssued es el comando que sale del evento InvoiceIssued v1: el documento comercial ya traducido al snapshot
// del comprobante, sin el emisor (sale del perfil fiscal, no del evento).
type InvoiceIssued struct {
	Document document.Document
}

// DocumentStore es fiscal.electronic_documents con sus líneas, impuestos, medios de pago e historial.
type DocumentStore interface {
	// ExistsBySource indica si la organización ya tiene un comprobante para ese documento comercial.
	ExistsBySource(ctx context.Context, sourceType document.SourceType, sourceDocumentID uuid.UUID) (bool, error)
	// Create guarda el documento en processing con su snapshot y la primera fila del historial.
	Create(ctx context.Context, d document.Document) error
}

// CreateDocumentFromInvoice crea el documento electrónico de una factura emitida (planning P5, F3), en processing y
// listo para el worker. El reproceso del mismo evento lo corta el inbox antes de llegar aquí (criterio 3); otro
// evento del mismo documento comercial se rechaza (UK electronic_documents_source_document_uk).
type CreateDocumentFromInvoice struct{}

func NewCreateDocumentFromInvoice() *CreateDocumentFromInvoice { return &CreateDocumentFromInvoice{} }

func (h *CreateDocumentFromInvoice) Handle(ctx context.Context, tx EventTx, ev IncomingEvent) error {
	cmd, ok := ev.Body.(InvoiceIssued)
	if !ok {
		return fmt.Errorf("%w: cuerpo inesperado %T", ErrInvalidEvent, ev.Body)
	}
	d := cmd.Document
	d.OrganizationID = ev.Ref.OrganizationID
	d.SourceEventID = ev.Ref.ID
	d.SourceType = document.SourceInvoice

	exists, err := tx.Documents().ExistsBySource(ctx, d.SourceType, d.SourceDocumentID)
	if err != nil {
		return err
	}
	if exists {
		return reject(fmt.Errorf("la factura %s ya tiene documento electrónico (llegó otro evento %s)",
			d.SourceDocumentNumber, ev.Ref.ID))
	}

	profile, err := tx.FiscalProfiles().Get(ctx, d.OrganizationID)
	if errors.Is(err, ErrNotFound) {
		// Sin perfil no hay emisor. Va a dead letter con un motivo claro; se reprocesa al configurar el perfil.
		// TODO(fiscal): ¿documento en error visible en la bandeja en vez de dead letter? (criterio 10)
		return reject(errors.New("la organización no configuró su perfil fiscal: no hay datos del emisor"))
	}
	if err != nil {
		return err
	}
	d.Issuer = issuerFrom(profile)

	d, err = document.New(d)
	if err != nil {
		return reject(err)
	}
	if err := tx.Documents().Create(ctx, d); err != nil {
		return err
	}
	return tx.Audit().Record(ctx, AuditEvent{
		OrganizationID: d.OrganizationID,
		ActorType:      ActorService,
		Action:         "electronic_document.created",
		EntityType:     "electronic_document",
		EntityID:       d.ID,
		CorrelationID:  ev.Ref.CorrelationID,
		Payload: map[string]any{
			"sourceType": string(d.SourceType), "sourceDocumentId": d.SourceDocumentID.String(),
			"sourceDocumentNumber": d.SourceDocumentNumber, "eventId": ev.Ref.ID.String(),
			"documentTypeCode": d.TypeCode(), "total": d.Total.String(), "currency": d.CurrencyCode,
		},
	})
}

// issuerFrom es el snapshot del emisor (Anexo 1 págs. 19–22) tomado del perfil fiscal al recibir el evento: un
// cambio posterior del perfil no altera el comprobante.
func issuerFrom(p FiscalProfile) document.Party {
	return document.Party{
		IdentificationTypeCode: p.IdentificationTypeCode,
		IdentificationNumber:   p.IdentificationNumber,
		LegalName:              p.LegalName,
		TradeName:              p.TradeName,
		EconomicActivityCode:   p.EconomicActivityCode,
		Email:                  p.Email,
		Phone:                  p.Phone,
		ProvinceCode:           p.ProvinceCode,
		CantonCode:             p.CantonCode,
		DistrictCode:           p.DistrictCode,
		Address:                p.AddressDetails,
	}
}
