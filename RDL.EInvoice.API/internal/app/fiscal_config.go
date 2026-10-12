package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"rdl/einvoice-api/internal/domain/certificate"
	"rdl/einvoice-api/internal/domain/permission"
	"rdl/einvoice-api/internal/domain/taxpayer"
	"rdl/einvoice-api/pkg/tenancy"
)

// Configuración fiscal (planning P5, F4; incremento 4): perfil del contribuyente, credenciales de Hacienda y
// certificado. Los secretos van al SecretStore y la base guarda solo sus referencias (informe 0002 §2.3).

var (
	// ErrProfileMissing: la operación exige el perfil fiscal configurado (problem fiscal-profile-missing, 409).
	ErrProfileMissing = errors.New("la organización no configuró su perfil fiscal")
	// ErrSecretsUnavailable: el SecretStore no está configurado o no respondió (503).
	ErrSecretsUnavailable = errors.New("el almacén de secretos no está disponible")
)

// ValidationErrors son varios campos inválidos de una petición (422).
type ValidationErrors []ValidationError

func (e ValidationErrors) Error() string {
	parts := make([]string, len(e))
	for i, v := range e {
		parts[i] = v.Error()
	}
	return strings.Join(parts, "; ")
}

// SecretStore guarda secretos cifrados ligados a una organización (decisión 6). Lo que entra nunca se registra en
// logs, auditoría ni respuestas.
type SecretStore interface {
	Put(ctx context.Context, organizationID uuid.UUID, data []byte) (uuid.UUID, error)
	Get(ctx context.Context, organizationID, id uuid.UUID) ([]byte, error)
	Delete(ctx context.Context, organizationID, id uuid.UUID) error
}

// CertificateReader abre un .p12 con su PIN y devuelve los datos públicos del certificado.
type CertificateReader interface {
	Inspect(p12 []byte, pin string) (certificate.Info, error)
}

// FiscalConfigStore escribe la configuración fiscal en la transacción del tenant.
type FiscalConfigStore interface {
	UpsertProfile(ctx context.Context, organizationID uuid.UUID, p taxpayer.Profile) (created bool, err error)
	IdentificationTypeExists(ctx context.Context, code string) (bool, error)
	DistrictExists(ctx context.Context, province, canton, district string) (bool, error)
	EconomicActivityExists(ctx context.Context, code string) (bool, error)
	// SetHaciendaCredentials guarda las referencias nuevas y devuelve las anteriores (uuid.Nil si no había).
	SetHaciendaCredentials(ctx context.Context, organizationID, usernameRef, passwordRef uuid.UUID) (prevUser, prevPass uuid.UUID, err error)
	// ReplaceCertificate desactiva el certificado activo y registra el nuevo (un solo activo por organización).
	ReplaceCertificate(ctx context.Context, c CertificateRecord) error
}

// CertificateRecord es la fila de fiscal.certificates: datos públicos y referencias al SecretStore.
type CertificateRecord struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	FileRef        uuid.UUID
	PINRef         uuid.UUID
	Info           certificate.Info
	UploadedBy     uuid.UUID
}

// --- PUT /v1/fiscal-profile ---

type UpdateFiscalProfile struct{ txm TxManager }

func NewUpdateFiscalProfile(txm TxManager) *UpdateFiscalProfile {
	return &UpdateFiscalProfile{txm: txm}
}

// Execute valida el perfil (formato del Anexo 1 y existencia en catálogos), lo guarda y devuelve cómo quedó.
func (uc *UpdateFiscalProfile) Execute(ctx context.Context, t tenancy.Context, p taxpayer.Profile, correlationID uuid.UUID) (FiscalProfile, error) {
	if err := authorize(t, permission.FiscalProfileWrite); err != nil {
		return FiscalProfile{}, err
	}
	if errs := p.Validate(); len(errs) > 0 {
		return FiscalProfile{}, toValidation(errs)
	}
	var out FiscalProfile
	err := uc.txm.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		store := tx.FiscalConfig()
		if verrs, err := checkCatalogs(ctx, store, p); err != nil || len(verrs) > 0 {
			if err != nil {
				return err
			}
			return verrs
		}
		created, err := store.UpsertProfile(ctx, t.OrganizationID(), p)
		if err != nil {
			return err
		}
		action := "fiscal.profile.updated"
		if created {
			action = "fiscal.profile.created"
		}
		if err := tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: t.OrganizationID(), ActorType: ActorUser, ActorUserID: t.UserID(), Action: action,
			EntityType: "taxpayer_profile", CorrelationID: correlationID,
			Payload: map[string]any{
				"environment": string(p.Environment), "identificationTypeCode": p.IdentificationTypeCode,
				"identificationNumber": p.IdentificationNumber, "legalName": p.LegalName,
				"economicActivityCode": p.EconomicActivityCode, "hasLocation": p.Location != nil,
			},
		}); err != nil {
			return err
		}
		out, err = tx.FiscalProfiles().Get(ctx, t.OrganizationID())
		return err
	})
	return out, err
}

// checkCatalogs verifica contra la base lo que el formato no alcanza: tipo de identificación (Nota 4), ubicación
// (Nota 14) y actividad económica (padrón del RUT). Catálogo vacío = el código no existe: se avisa en el mensaje.
func checkCatalogs(ctx context.Context, s FiscalConfigStore, p taxpayer.Profile) (ValidationErrors, error) {
	var errs ValidationErrors
	ok, err := s.IdentificationTypeExists(ctx, p.IdentificationTypeCode)
	if err != nil {
		return nil, err
	}
	if !ok {
		errs = append(errs, ValidationError{Field: "identification.typeCode", Message: "no está en el catálogo de tipos de identificación (Nota 4)"})
	}
	if l := p.Location; l != nil {
		ok, err := s.DistrictExists(ctx, l.ProvinceCode, l.CantonCode, l.DistrictCode)
		if err != nil {
			return nil, err
		}
		if !ok {
			errs = append(errs, ValidationError{Field: "location", Message: "provincia, cantón y distrito no están en el catálogo de ubicación (Nota 14; todavía sin cargar)"})
		}
	}
	if p.EconomicActivityCode != "" {
		ok, err := s.EconomicActivityExists(ctx, p.EconomicActivityCode)
		if err != nil {
			return nil, err
		}
		if !ok {
			errs = append(errs, ValidationError{Field: "economicActivityCode", Message: "no está en el catálogo de actividades económicas (padrón del RUT; todavía sin cargar)"})
		}
	}
	return errs, nil
}

func toValidation(errs []taxpayer.FieldError) ValidationErrors {
	out := make(ValidationErrors, len(errs))
	for i, e := range errs {
		out[i] = ValidationError{Field: e.Field, Message: e.Message}
	}
	return out
}

// --- PUT /v1/fiscal-profile/hacienda-credentials ---

// HaciendaCredentials son el usuario y la contraseña del API de Hacienda que el contribuyente genera en ATV
// (Anexo 3, pág. 98). Se guardan en el SecretStore; jamás en la base, logs ni auditoría.
type HaciendaCredentials struct {
	Username string
	Password string
}

type SetHaciendaCredentials struct {
	txm     TxManager
	secrets SecretStore
}

func NewSetHaciendaCredentials(txm TxManager, secrets SecretStore) *SetHaciendaCredentials {
	return &SetHaciendaCredentials{txm: txm, secrets: secrets}
}

func (uc *SetHaciendaCredentials) Execute(ctx context.Context, t tenancy.Context, c HaciendaCredentials, correlationID uuid.UUID) error {
	if err := authorize(t, permission.HaciendaCredentialsWrite); err != nil {
		return err
	}
	var errs ValidationErrors
	// Usuario: «Identificación del contribuyente. Ej: cpf-01-1234-5678@comprobanteselectronicos.go.cr» (pág. 98).
	if n := utf8.RuneCountInString(c.Username); n < 3 || n > 160 || !strings.Contains(c.Username, "@") {
		errs = append(errs, ValidationError{Field: "username", Message: "debe ser el usuario del API que entrega ATV (con @)"})
	}
	if n := len(c.Password); n == 0 || n > 512 {
		errs = append(errs, ValidationError{Field: "password", Message: "es obligatoria"})
	}
	if len(errs) > 0 {
		return errs
	}
	if uc.secrets == nil {
		return ErrSecretsUnavailable
	}
	org := t.OrganizationID()
	var prevUser, prevPass uuid.UUID
	var created []uuid.UUID
	err := uc.txm.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		if _, err := tx.FiscalProfiles().Get(ctx, org); err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrProfileMissing
			}
			return err
		}
		userRef, err := uc.secrets.Put(ctx, org, []byte(c.Username))
		if err != nil {
			return fmt.Errorf("%w: %w", ErrSecretsUnavailable, err)
		}
		created = append(created, userRef)
		passRef, err := uc.secrets.Put(ctx, org, []byte(c.Password))
		if err != nil {
			return fmt.Errorf("%w: %w", ErrSecretsUnavailable, err)
		}
		created = append(created, passRef)
		prevUser, prevPass, err = tx.FiscalConfig().SetHaciendaCredentials(ctx, org, userRef, passRef)
		if err != nil {
			return err
		}
		return tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: org, ActorType: ActorUser, ActorUserID: t.UserID(),
			Action: "fiscal.hacienda_credentials.updated", EntityType: "taxpayer_profile", CorrelationID: correlationID,
			Payload: map[string]any{"replaced": prevUser != uuid.Nil},
		})
	})
	if err != nil {
		uc.discard(org, created)
		return err
	}
	// Las anteriores ya no se usan: se borran después del commit (si falla, quedan huérfanas pero cifradas).
	uc.discard(org, []uuid.UUID{prevUser, prevPass})
	return nil
}

func (uc *SetHaciendaCredentials) discard(org uuid.UUID, ids []uuid.UUID) {
	discardSecrets(uc.secrets, org, ids)
}

func discardSecrets(s SecretStore, org uuid.UUID, ids []uuid.UUID) {
	for _, id := range ids {
		if id != uuid.Nil {
			_ = s.Delete(context.Background(), org, id)
		}
	}
}

// --- PUT /v1/fiscal-profile/certificate ---

// ErrCertificateInvalid y ErrCertificateExpired son los problem types certificate-invalid y certificate-expired.
var (
	ErrCertificateInvalid = errors.New("certificado inválido")
	ErrCertificateExpired = errors.New("certificado vencido o todavía no vigente")
)

type UploadCertificate struct {
	txm     TxManager
	secrets SecretStore
	reader  CertificateReader
	now     func() time.Time
}

func NewUploadCertificate(txm TxManager, secrets SecretStore, reader CertificateReader) *UploadCertificate {
	return &UploadCertificate{txm: txm, secrets: secrets, reader: reader, now: time.Now}
}

// Execute abre el .p12 con el PIN, verifica llave y vigencia (Anexo 2), guarda archivo y PIN en el SecretStore y
// deja este certificado como el único activo de la organización. Ni el archivo ni el PIN salen de aquí.
func (uc *UploadCertificate) Execute(ctx context.Context, t tenancy.Context, p12 []byte, pin string, correlationID uuid.UUID) (certificate.Info, error) {
	if err := authorize(t, permission.CertificateUpload); err != nil {
		return certificate.Info{}, err
	}
	info, err := uc.reader.Inspect(p12, pin)
	if err != nil {
		return certificate.Info{}, fmt.Errorf("%w: %w", ErrCertificateInvalid, certificate.ErrUnreadable)
	}
	switch err := certificate.Validate(info, uc.now()); {
	case errors.Is(err, certificate.ErrExpired), errors.Is(err, certificate.ErrNotYetValid):
		return certificate.Info{}, fmt.Errorf("%w: %w", ErrCertificateExpired, err)
	case err != nil:
		return certificate.Info{}, fmt.Errorf("%w: %w", ErrCertificateInvalid, err)
	}
	if uc.secrets == nil {
		return certificate.Info{}, ErrSecretsUnavailable
	}
	org := t.OrganizationID()
	var created []uuid.UUID
	err = uc.txm.WithinTenantTx(ctx, t, func(ctx context.Context, tx Tx) error {
		if _, err := tx.FiscalProfiles().Get(ctx, org); err != nil {
			if errors.Is(err, ErrNotFound) {
				return ErrProfileMissing
			}
			return err
		}
		fileRef, err := uc.secrets.Put(ctx, org, p12)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrSecretsUnavailable, err)
		}
		created = append(created, fileRef)
		pinRef, err := uc.secrets.Put(ctx, org, []byte(pin))
		if err != nil {
			return fmt.Errorf("%w: %w", ErrSecretsUnavailable, err)
		}
		created = append(created, pinRef)
		rec := CertificateRecord{ID: uuid.New(), OrganizationID: org, FileRef: fileRef, PINRef: pinRef, Info: info, UploadedBy: t.UserID()}
		if err := tx.FiscalConfig().ReplaceCertificate(ctx, rec); err != nil {
			return err
		}
		// TODO(fiscal): los secretos del certificado anterior quedan (cifrados) con su fila inactiva; definir si se
		// borran al reemplazarlo o se conservan por la retención (decisión 7).
		return tx.Audit().Record(ctx, AuditEvent{
			OrganizationID: org, ActorType: ActorUser, ActorUserID: t.UserID(), Action: "fiscal.certificate.uploaded",
			EntityType: "certificate", EntityID: rec.ID, CorrelationID: correlationID,
			Payload: map[string]any{
				"subject": info.Subject, "issuer": info.Issuer, "serialNumber": info.SerialNumber,
				"validFrom": info.NotBefore.Format(time.RFC3339), "validTo": info.NotAfter.Format(time.RFC3339),
				"keyBits": info.KeyBits,
			},
		})
	})
	if err != nil {
		discardSecrets(uc.secrets, org, created)
		return certificate.Info{}, err
	}
	return info, nil
}
