package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"rdl/einvoice-api/internal/adapters/pkcs12"
	"rdl/einvoice-api/internal/app"
	"rdl/einvoice-api/internal/domain/certificate"
	"rdl/einvoice-api/internal/domain/taxpayer"
	"rdl/einvoice-api/pkg/correlation"
	"rdl/einvoice-api/pkg/tenancy"
)

// FiscalConfigHandlers son las escrituras de /v1/fiscal-profile (incremento 4).
type FiscalConfigHandlers struct {
	Update interface {
		Execute(ctx context.Context, t tenancy.Context, p taxpayer.Profile, correlationID uuid.UUID) (app.FiscalProfile, error)
	}
	Credentials interface {
		Execute(ctx context.Context, t tenancy.Context, c app.HaciendaCredentials, correlationID uuid.UUID) error
	}
	Certificate interface {
		Execute(ctx context.Context, t tenancy.Context, p12 []byte, pin string, correlationID uuid.UUID) (certificate.Info, error)
	}
}

// fiscalProfileRequest es FiscalProfile de api/openapi.yaml sin los campos de solo lectura.
type fiscalProfileRequest struct {
	Environment          string            `json:"environment"`
	Identification       identificationDTO `json:"identification"`
	LegalName            string            `json:"legalName"`
	TradeName            string            `json:"tradeName"`
	Email                string            `json:"email"`
	Phone                string            `json:"phone"`
	Location             *locationDTO      `json:"location"`
	EconomicActivityCode string            `json:"economicActivityCode"`
}

type haciendaCredentialsRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Tope del formulario del certificado: el .p12 (pkcs12.MaxFileBytes) más el PIN y los encabezados multipart.
const maxCertificateForm = pkcs12.MaxFileBytes + 8<<10

func (h *FiscalConfigHandlers) register(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("PUT /v1/fiscal-profile", func(w http.ResponseWriter, r *http.Request) {
		var in fiscalProfileRequest
		if err := decodeJSON(w, r, &in); err != nil {
			fail(w, r, err)
			return
		}
		p := taxpayer.Profile{
			Environment:            taxpayer.Environment(in.Environment),
			IdentificationTypeCode: strings.TrimSpace(in.Identification.TypeCode),
			IdentificationNumber:   strings.TrimSpace(in.Identification.Number),
			LegalName:              strings.TrimSpace(in.LegalName),
			TradeName:              strings.TrimSpace(in.TradeName),
			Email:                  strings.TrimSpace(in.Email),
			Phone:                  strings.TrimSpace(in.Phone),
			EconomicActivityCode:   strings.TrimSpace(in.EconomicActivityCode),
		}
		if l := in.Location; l != nil {
			p.Location = &taxpayer.Location{ProvinceCode: l.ProvinceCode, CantonCode: l.CantonCode, DistrictCode: l.DistrictCode,
				AddressDetails: strings.TrimSpace(l.AddressDetails)}
		}
		t, _ := tenancy.From(r.Context())
		out, err := h.Update.Execute(r.Context(), t, p, correlationID(r))
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toFiscalProfileDTO(out))
	})

	rt.handle("PUT /v1/fiscal-profile/hacienda-credentials", func(w http.ResponseWriter, r *http.Request) {
		var in haciendaCredentialsRequest
		if err := decodeJSON(w, r, &in); err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		if err := h.Credentials.Execute(r.Context(), t, app.HaciendaCredentials{Username: strings.TrimSpace(in.Username), Password: in.Password}, correlationID(r)); err != nil {
			fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	rt.handle("PUT /v1/fiscal-profile/certificate", func(w http.ResponseWriter, r *http.Request) {
		p12, pin, err := readCertificateForm(w, r)
		if err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		if _, err := h.Certificate.Execute(r.Context(), t, p12, pin, correlationID(r)); err != nil {
			fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// readCertificateForm lee multipart/form-data con las partes certificate (.p12) y pin, en memoria y con tope. No usa
// ParseMultipartForm: ese método puede volcar el archivo a un temporal en disco, y la llave privada no toca el disco
// en claro (prompt P5, Paso 3).
func readCertificateForm(w http.ResponseWriter, r *http.Request) ([]byte, string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCertificateForm)
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, "", fmt.Errorf("%w: se esperaba multipart/form-data con certificate y pin", errMalformed)
	}
	var p12 []byte
	var pin string
	var gotFile, gotPIN bool
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("%w: %s", errMalformed, describeJSONError(err))
		}
		var buf bytes.Buffer
		if _, err := io.Copy(&buf, io.LimitReader(part, pkcs12.MaxFileBytes+1)); err != nil {
			_ = part.Close()
			return nil, "", fmt.Errorf("%w: el formulario supera el tamaño máximo", errMalformed)
		}
		_ = part.Close()
		if buf.Len() > pkcs12.MaxFileBytes {
			return nil, "", fmt.Errorf("%w: el certificado supera %d KiB", errMalformed, pkcs12.MaxFileBytes>>10)
		}
		switch part.FormName() {
		case "certificate":
			p12, gotFile = buf.Bytes(), true
		case "pin":
			pin, gotPIN = buf.String(), true
		default:
			return nil, "", fmt.Errorf("%w: parte no permitida %q", errMalformed, part.FormName())
		}
	}
	if !gotFile || !gotPIN {
		var f fields
		if !gotFile {
			f.add("certificate", "falta el archivo .p12")
		}
		if !gotPIN {
			f.add("pin", "falta el PIN")
		}
		return nil, "", f.err()
	}
	return p12, pin, nil
}

// correlationID es el X-Correlation-Id del request (lo fija el middleware); va a la auditoría.
func correlationID(r *http.Request) uuid.UUID {
	id, _ := correlation.FromContext(r.Context())
	return id
}
