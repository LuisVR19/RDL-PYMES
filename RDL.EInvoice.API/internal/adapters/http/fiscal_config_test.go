package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/einvoice-api/internal/app"
	"rdl/einvoice-api/internal/domain/certificate"
	"rdl/einvoice-api/internal/domain/taxpayer"
	"rdl/einvoice-api/internal/platform/health"
	"rdl/einvoice-api/pkg/tenancy"
)

type fakeConfig struct {
	err     error
	gotPIN  string
	gotFile []byte
}

func (f *fakeConfig) Execute(_ context.Context, _ tenancy.Context, p12 []byte, pin string, _ uuid.UUID) (certificate.Info, error) {
	f.gotFile, f.gotPIN = p12, pin
	return certificate.Info{}, f.err
}

type fakeUpdate struct{ err error }

func (f fakeUpdate) Execute(context.Context, tenancy.Context, taxpayer.Profile, uuid.UUID) (app.FiscalProfile, error) {
	return app.FiscalProfile{Environment: "sandbox", UpdatedAt: time.Now()}, f.err
}

type fakeCredentials struct{ err error }

func (f fakeCredentials) Execute(context.Context, tenancy.Context, app.HaciendaCredentials, uuid.UUID) error {
	return f.err
}

func configRouter(cert *fakeConfig, upd fakeUpdate, cred fakeCredentials) http.Handler {
	org := uuid.New()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewRouter(Deps{
		Log: log, Health: health.New(log, time.Second),
		Verifier:     fakeVerifier{"owner": {Subject: "s", OrganizationID: org}},
		Memberships:  fakeMemberships{"s": {UserID: uuid.New(), Roles: []string{"owner"}}},
		FiscalConfig: &FiscalConfigHandlers{Update: upd, Credentials: cred, Certificate: cert},
	})
}

func form(t *testing.T, parts map[string]string) (string, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for name, v := range parts {
		w, _ := mw.CreateFormField(name)
		_, _ = w.Write([]byte(v))
	}
	_ = mw.Close()
	return mw.FormDataContentType(), &buf
}

func do(t *testing.T, h http.Handler, method, path, ct string, body io.Reader) (int, map[string]any, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Authorization", "Bearer owner")
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	return rec.Code, m, rec.Body.String()
}

func TestCertificateFormAndErrors(t *testing.T) {
	cert := &fakeConfig{}
	h := configRouter(cert, fakeUpdate{}, fakeCredentials{})

	ct, body := form(t, map[string]string{"certificate": "P12BYTES", "pin": "9876"})
	if code, _, _ := do(t, h, http.MethodPut, "/v1/fiscal-profile/certificate", ct, body); code != http.StatusNoContent {
		t.Fatalf("formulario válido: %d", code)
	}
	if string(cert.gotFile) != "P12BYTES" || cert.gotPIN != "9876" {
		t.Errorf("partes leídas: %q %q", cert.gotFile, cert.gotPIN)
	}

	ct, body = form(t, map[string]string{"certificate": "P12BYTES"})
	if code, m, _ := do(t, h, http.MethodPut, "/v1/fiscal-profile/certificate", ct, body); code != http.StatusUnprocessableEntity ||
		!strings.Contains(fmt.Sprint(m["errors"]), "pin") {
		t.Errorf("sin PIN: %d %v", code, m)
	}
	if code, m, _ := do(t, h, http.MethodPut, "/v1/fiscal-profile/certificate", "application/json", strings.NewReader("{}")); code != http.StatusBadRequest ||
		m["type"] != "urn:rdl:fiscal:problem:malformed-request" {
		t.Errorf("sin multipart: %d %v", code, m)
	}
	ct, body = form(t, map[string]string{"certificate": strings.Repeat("x", 70<<10), "pin": "1"})
	if code, _, _ := do(t, h, http.MethodPut, "/v1/fiscal-profile/certificate", ct, body); code != http.StatusBadRequest {
		t.Errorf("archivo demasiado grande: %d", code)
	}

	for err, want := range map[error]string{
		fmt.Errorf("%w: %w", app.ErrCertificateInvalid, certificate.ErrUnreadable): "certificate-invalid",
		fmt.Errorf("%w: %w", app.ErrCertificateExpired, certificate.ErrExpired):    "certificate-expired",
		app.ErrProfileMissing:     "fiscal-profile-missing",
		app.ErrSecretsUnavailable: "unavailable",
	} {
		cert.err = err
		ct, body := form(t, map[string]string{"certificate": "x", "pin": "9876"})
		_, m, raw := do(t, h, http.MethodPut, "/v1/fiscal-profile/certificate", ct, body)
		if m["type"] != "urn:rdl:fiscal:problem:"+want {
			t.Errorf("%v: type = %v", err, m["type"])
		}
		if strings.Contains(raw, "9876") {
			t.Errorf("%v: la respuesta contiene el PIN", err)
		}
	}
}

func TestProfileAndCredentialsErrors(t *testing.T) {
	h := configRouter(&fakeConfig{}, fakeUpdate{err: app.ValidationErrors{
		{Field: "environment", Message: "solo sandbox"}, {Field: "email", Message: "inválido"},
	}}, fakeCredentials{err: app.ErrProfileMissing})

	code, m, _ := do(t, h, http.MethodPut, "/v1/fiscal-profile", "application/json", strings.NewReader(`{"environment":"production"}`))
	if code != http.StatusUnprocessableEntity || len(m["errors"].([]any)) != 2 {
		t.Errorf("perfil inválido: %d %v", code, m)
	}
	code, m, _ = do(t, h, http.MethodPut, "/v1/fiscal-profile", "application/json", strings.NewReader(`{"otro":1}`))
	if code != http.StatusBadRequest {
		t.Errorf("campo desconocido: %d %v", code, m)
	}
	code, m, raw := do(t, h, http.MethodPut, "/v1/fiscal-profile/hacienda-credentials", "application/json",
		strings.NewReader(`{"username":"u@x","password":"secreta-123"}`))
	if code != http.StatusConflict || m["type"] != "urn:rdl:fiscal:problem:fiscal-profile-missing" || strings.Contains(raw, "secreta-123") {
		t.Errorf("credenciales sin perfil: %d %v", code, m)
	}
}
