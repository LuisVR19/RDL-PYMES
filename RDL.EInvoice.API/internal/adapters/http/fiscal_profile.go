package http

import (
	"context"
	"net/http"
	"time"

	"rdl/einvoice-api/internal/app"
	"rdl/einvoice-api/pkg/tenancy"
)

// FiscalProfileHandlers agrupa /v1/fiscal-profile. PUT y el certificado llegan con el incremento 4.
type FiscalProfileHandlers struct {
	Get interface {
		Execute(ctx context.Context, t tenancy.Context) (app.FiscalProfile, error)
	}
}

func (h *FiscalProfileHandlers) register(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("GET /v1/fiscal-profile", func(w http.ResponseWriter, r *http.Request) {
		t, _ := tenancy.From(r.Context()) // el guard de /v1 garantiza el TenantContext
		p, err := h.Get.Execute(r.Context(), t)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toFiscalProfileDTO(p))
	})
}

// fiscalProfileDTO es FiscalProfile de api/openapi.yaml: amplía el esqueleto de RDL.Contracts/openapi/fiscal.yaml
// (environment, economicActivityCode, certificateValidTo) con los campos del contribuyente. Sin secretos.
type fiscalProfileDTO struct {
	Environment            string            `json:"environment"`
	Identification         identificationDTO `json:"identification"`
	LegalName              string            `json:"legalName"`
	TradeName              string            `json:"tradeName,omitempty"`
	Email                  string            `json:"email"`
	Phone                  string            `json:"phone,omitempty"`
	Location               *locationDTO      `json:"location,omitempty"`
	EconomicActivityCode   string            `json:"economicActivityCode,omitempty"`
	HaciendaCredentialsSet bool              `json:"haciendaCredentialsSet"`
	CertificateValidTo     string            `json:"certificateValidTo,omitempty"`
	UpdatedAt              string            `json:"updatedAt"`
}

type identificationDTO struct {
	TypeCode string `json:"typeCode"`
	Number   string `json:"number"`
}

type locationDTO struct {
	ProvinceCode   string `json:"provinceCode"`
	CantonCode     string `json:"cantonCode"`
	DistrictCode   string `json:"districtCode"`
	AddressDetails string `json:"addressDetails,omitempty"`
}

func toFiscalProfileDTO(p app.FiscalProfile) fiscalProfileDTO {
	dto := fiscalProfileDTO{
		Environment:            p.Environment,
		Identification:         identificationDTO{TypeCode: p.IdentificationTypeCode, Number: p.IdentificationNumber},
		LegalName:              p.LegalName,
		TradeName:              p.TradeName,
		Email:                  p.Email,
		Phone:                  p.Phone,
		EconomicActivityCode:   p.EconomicActivityCode,
		HaciendaCredentialsSet: p.HaciendaCredentialsSet,
		UpdatedAt:              p.UpdatedAt.UTC().Format(time.RFC3339),
	}
	// La FK de la base exige los tres códigos juntos.
	if p.ProvinceCode != "" {
		dto.Location = &locationDTO{
			ProvinceCode: p.ProvinceCode, CantonCode: p.CantonCode, DistrictCode: p.DistrictCode,
			AddressDetails: p.AddressDetails,
		}
	}
	if p.CertificateValidTo != nil {
		dto.CertificateValidTo = p.CertificateValidTo.UTC().Format(time.RFC3339)
	}
	return dto
}
