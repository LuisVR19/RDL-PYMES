package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/einvoice-api/internal/app"
	"rdl/einvoice-api/internal/platform/health"
	"rdl/einvoice-api/pkg/tenancy"
)

// Tokens de prueba: el "token" es el nombre de un usuario de la tabla.
type fakeVerifier map[string]tenancy.Identity

func (f fakeVerifier) Verify(_ context.Context, raw string) (tenancy.Identity, error) {
	id, ok := f[raw]
	if !ok {
		return tenancy.Identity{}, errors.New("firma inválida")
	}
	return id, nil
}

type fakeMemberships map[string]tenancy.Membership // subject → membresía en la organización del token

func (f fakeMemberships) ActiveMembership(_ context.Context, subject string, _ uuid.UUID) (tenancy.Membership, error) {
	m, ok := f[subject]
	if !ok {
		return tenancy.Membership{}, tenancy.ErrNoMembership
	}
	return m, nil
}

type fakeProfileReader struct {
	profiles map[uuid.UUID]app.FiscalProfile
}

func (f fakeProfileReader) Execute(_ context.Context, t tenancy.Context) (app.FiscalProfile, error) {
	for _, r := range t.Roles() {
		if r == "owner" || r == "admin" || r == "accountant" {
			if p, ok := f.profiles[t.OrganizationID()]; ok {
				return p, nil
			}
			return app.FiscalProfile{}, app.ErrNotFound
		}
	}
	return app.FiscalProfile{}, app.ErrForbidden
}

func TestGetFiscalProfileHTTP(t *testing.T) {
	orgA, orgB := uuid.New(), uuid.New()
	validTo := time.Date(2027, 3, 1, 6, 0, 0, 0, time.UTC)
	reader := fakeProfileReader{profiles: map[uuid.UUID]app.FiscalProfile{orgA: {
		Environment: "sandbox", IdentificationTypeCode: "02", IdentificationNumber: "3101123456",
		LegalName: "PYME de prueba S.A.", Email: "facturas@example.test", ProvinceCode: "1", CantonCode: "01",
		DistrictCode: "01", EconomicActivityCode: "620100", HaciendaCredentialsSet: true,
		CertificateValidTo: &validTo, UpdatedAt: validTo,
	}}}
	tokens := fakeVerifier{
		"owner-a":      {Subject: "s-owner-a", OrganizationID: orgA},
		"biller-a":     {Subject: "s-biller-a", OrganizationID: orgA},
		"owner-b":      {Subject: "s-owner-b", OrganizationID: orgB},
		"sin-org":      {Subject: "s-owner-a"},
		"suspendido-a": {Subject: "s-suspendido", OrganizationID: orgA},
	}
	members := fakeMemberships{
		"s-owner-a":  {UserID: uuid.New(), Roles: []string{"owner"}},
		"s-biller-a": {UserID: uuid.New(), Roles: []string{"biller"}},
		"s-owner-b":  {UserID: uuid.New(), Roles: []string{"owner"}},
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := NewRouter(Deps{
		Log: log, Health: health.New(log, time.Second), Verifier: tokens, Memberships: members,
		FiscalProfile: &FiscalProfileHandlers{Get: reader},
	})

	cases := []struct {
		name, token, method string
		status              int
		problem             string
	}{
		{"sin token", "", http.MethodGet, 401, "unauthenticated"},
		{"token inválido", "otro", http.MethodGet, 401, "unauthenticated"},
		{"token sin org_id", "sin-org", http.MethodGet, 403, "no-active-organization"},
		{"membresía inactiva", "suspendido-a", http.MethodGet, 403, "membership-inactive"},
		{"rol sin permiso", "biller-a", http.MethodGet, 403, "forbidden"},
		{"organización sin perfil", "owner-b", http.MethodGet, 404, "not-found"},
		{"método no admitido", "owner-a", http.MethodDelete, 405, "method-not-allowed"},
		{"owner con perfil", "owner-a", http.MethodGet, 200, ""},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, "/v1/fiscal-profile", nil)
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != c.status {
			t.Errorf("%s: status = %d, se esperaba %d (%s)", c.name, rec.Code, c.status, rec.Body)
			continue
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: cuerpo no es JSON: %v", c.name, err)
		}
		if c.problem != "" {
			if body["type"] != "urn:rdl:fiscal:problem:"+c.problem {
				t.Errorf("%s: type = %v", c.name, body["type"])
			}
			continue
		}
		if body["environment"] != "sandbox" || body["certificateValidTo"] != "2027-03-01T06:00:00Z" ||
			body["haciendaCredentialsSet"] != true || body["economicActivityCode"] != "620100" {
			t.Errorf("%s: cuerpo = %s", c.name, rec.Body)
		}
		if loc, _ := body["location"].(map[string]any); loc["districtCode"] != "01" {
			t.Errorf("%s: location = %v", c.name, body["location"])
		}
		for _, secret := range []string{"secret", "password", "pin", "Secret"} {
			if strings.Contains(rec.Body.String(), secret) {
				t.Errorf("%s: la respuesta menciona %q", c.name, secret)
			}
		}
	}
}
