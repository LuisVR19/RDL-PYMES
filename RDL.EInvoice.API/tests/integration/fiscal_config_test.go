//go:build integration

package integration

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"rdl/einvoice-api/internal/testcert"
	"rdl/einvoice-api/tests/testkit"
)

func certificateForm(t *testing.T, p12 []byte, pin string) (string, []byte) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("certificate", "llave.p12")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(p12)
	_ = mw.WriteField("pin", pin)
	_ = mw.Close()
	return mw.FormDataContentType(), buf.Bytes()
}

func profileBody() map[string]any {
	return map[string]any{
		"environment":    "sandbox",
		"identification": map[string]any{"typeCode": "02", "number": "3101000001"},
		"legalName":      "Organización de prueba fiscal S.A.",
		"tradeName":      "Prueba Fiscal",
		"email":          "fiscal.pruebas@example.test",
		"phone":          "22223333",
	}
}

func TestFiscalProfileWrite(t *testing.T) {
	owner := env.A.OwnerToken(env)
	r := env.CallBody(t, http.MethodPut, "/v1/fiscal-profile", owner, profileBody())
	if r.Status != http.StatusOK || r.Str("legalName") != "Organización de prueba fiscal S.A." || r.Str("tradeName") != "Prueba Fiscal" {
		t.Fatalf("PUT perfil: %d %v", r.Status, r.Body)
	}

	// Producción, formato del anexo y catálogos sin cargar (ubicación y actividades): 422 con cada campo.
	bad := profileBody()
	bad["environment"] = "production"
	bad["identification"] = map[string]any{"typeCode": "02", "number": "3-101-000001"}
	bad["location"] = map[string]any{"provinceCode": "1", "cantonCode": "01", "districtCode": "01", "addressDetails": "Del parque 100 m al norte"}
	bad["economicActivityCode"] = "620100"
	r = env.CallBody(t, http.MethodPut, "/v1/fiscal-profile", owner, bad)
	if r.Status != http.StatusUnprocessableEntity || r.Str("type") != "urn:rdl:fiscal:problem:validation" {
		t.Fatalf("perfil inválido: %d %v", r.Status, r.Body)
	}
	fields := map[string]bool{}
	for _, e := range r.Body["errors"].([]any) {
		fields[e.(map[string]any)["field"].(string)] = true
	}
	if !fields["environment"] || !fields["identification.number"] {
		t.Errorf("campos de formato: %v", fields)
	}
	// Sin errores de formato se verifican los catálogos: la ubicación existe (Codificacionubicacion_V4.4, incremento
	// 5); el padrón de actividades económicas todavía no está cargado.
	bad = profileBody()
	bad["location"] = map[string]any{"provinceCode": "1", "cantonCode": "01", "districtCode": "01", "addressDetails": "Del parque 100 m al norte"}
	bad["economicActivityCode"] = "620100"
	r = env.CallBody(t, http.MethodPut, "/v1/fiscal-profile", owner, bad)
	if errs, _ := r.Body["errors"].([]any); r.Status != http.StatusUnprocessableEntity || len(errs) != 1 ||
		errs[0].(map[string]any)["field"] != "economicActivityCode" {
		t.Errorf("catálogos: %d %v", r.Status, r.Body)
	}
	// Distrito que no existe en el catálogo de ubicación.
	bad["economicActivityCode"] = ""
	bad["location"] = map[string]any{"provinceCode": "1", "cantonCode": "01", "districtCode": "99", "addressDetails": "Del parque 100 m al norte"}
	if r = env.CallBody(t, http.MethodPut, "/v1/fiscal-profile", owner, bad); r.Status != http.StatusUnprocessableEntity {
		t.Errorf("distrito inexistente: %d %v", r.Status, r.Body)
	}
	// Con ubicación válida el perfil se guarda y la devuelve.
	ok := profileBody()
	ok["location"] = map[string]any{"provinceCode": "1", "cantonCode": "01", "districtCode": "01", "addressDetails": "Del parque 100 m al norte"}
	r = env.CallBody(t, http.MethodPut, "/v1/fiscal-profile", owner, ok)
	if loc, _ := r.Body["location"].(map[string]any); r.Status != http.StatusOK || loc["districtCode"] != "01" {
		t.Errorf("perfil con ubicación: %d %v", r.Status, r.Body)
	}

	// B no ve el perfil de A: su GET responde con el suyo o 404, nunca el de A.
	r = env.Call(t, http.MethodGet, "/v1/fiscal-profile", env.B.OwnerToken(env))
	if r.Str("legalName") == "Organización de prueba fiscal S.A." {
		t.Error("B leyó el perfil de A")
	}
}

func TestHaciendaCredentialsAndCertificate(t *testing.T) {
	ctx := context.Background()
	env.EnsureProfile(ctx, t, env.A)
	owner := env.A.OwnerToken(env)
	const password = "contraseña-de-prueba-NO-real-9f3c" // #nosec G101 -- valor de prueba inventado, se busca en las salidas
	const pin = "4321"

	r := env.CallBody(t, http.MethodPut, "/v1/fiscal-profile/hacienda-credentials", owner,
		map[string]any{"username": "cpj-3-101-000001@prod.comprobanteselectronicos.go.cr", "password": password})
	if r.Status != http.StatusNoContent {
		t.Fatalf("credenciales: %d %v", r.Status, r.Body)
	}

	now := time.Now()
	valid := testcert.P12(t, testcert.Options{Bits: 2048, NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(1, 0, 0), PIN: pin})
	expired := testcert.P12(t, testcert.Options{Bits: 2048, NotBefore: now.AddDate(-2, 0, 0), NotAfter: now.AddDate(-1, 0, 0), PIN: pin})
	for _, c := range []struct {
		name, pin, problem string
		p12                []byte
	}{
		{"PIN incorrecto", "0000", "certificate-invalid", valid},
		{"vencido", pin, "certificate-expired", expired},
	} {
		ct, body := certificateForm(t, c.p12, c.pin)
		r := env.CallRaw(t, http.MethodPut, "/v1/fiscal-profile/certificate", owner, ct, body)
		if r.Status != http.StatusUnprocessableEntity || r.Str("type") != "urn:rdl:fiscal:problem:"+c.problem {
			t.Errorf("%s: %d %v", c.name, r.Status, r.Body)
		}
	}
	for i := 0; i < 2; i++ { // dos veces: el segundo reemplaza al primero, y solo uno queda activo
		ct, body := certificateForm(t, valid, pin)
		if r := env.CallRaw(t, http.MethodPut, "/v1/fiscal-profile/certificate", owner, ct, body); r.Status != http.StatusNoContent {
			t.Fatalf("certificado válido (%d): %d %v", i, r.Status, r.Body)
		}
	}

	r = env.Call(t, http.MethodGet, "/v1/fiscal-profile", owner)
	if r.Status != http.StatusOK || r.Body["haciendaCredentialsSet"] != true || r.Str("certificateValidTo") == "" {
		t.Errorf("perfil después de configurar: %d %v", r.Status, r.Body)
	}

	// En la base: un solo certificado activo, referencias al SecretStore y ningún secreto en claro (ni en la
	// auditoría de la organización).
	err := testkit.WithSession(ctx, env.Pool, env.A.Org, uuid.Nil, func(tx pgx.Tx) error {
		var active int
		var path string
		if err := tx.QueryRow(ctx, `select count(*), max(storage_path) from fiscal.certificates
			where organization_id = $1 and is_active`, env.A.Org).Scan(&active, &path); err != nil {
			return err
		}
		if active != 1 || !strings.HasPrefix(path, "secret://") {
			t.Errorf("certificados activos = %d, storage_path = %q", active, path)
		}
		var leaks int
		if err := tx.QueryRow(ctx, `select count(*) from audit.audit_events
			where organization_id = $1 and service = 'fiscal' and occurred_at > now() - interval '10 minutes'
			  and (payload::text like '%' || $2 || '%' or payload::text like '%' || $3 || '%')`,
			env.A.Org, password, `"`+pin+`"`).Scan(&leaks); err != nil {
			return err
		}
		if leaks != 0 {
			t.Errorf("%d audit events contienen la contraseña o el PIN", leaks)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// En disco: archivos cifrados, solo de A, sin la contraseña ni el PIN en claro.
	files, _ := filepath.Glob(filepath.Join(env.SecretsDir, env.A.Org.String(), "*.bin"))
	if len(files) == 0 {
		t.Fatal("no hay secretos de A en el SecretStore")
	}
	for _, f := range files {
		raw, _ := os.ReadFile(f) //nolint:gosec // G304: archivos del directorio temporal de la suite
		if bytes.Contains(raw, []byte(password)) || bytes.Contains(raw, valid[:32]) {
			t.Errorf("%s guarda un secreto en claro", filepath.Base(f))
		}
	}
	if others, _ := filepath.Glob(filepath.Join(env.SecretsDir, env.B.Org.String(), "*.bin")); len(others) != 0 {
		t.Errorf("hay secretos en la carpeta de B: %v", others)
	}

	// B no configura nada en A ni la ve: sus cambios van a su propia organización (sin perfil → 409).
	ct, body := certificateForm(t, valid, pin)
	r = env.CallRaw(t, http.MethodPut, "/v1/fiscal-profile/certificate", env.B.OwnerToken(env), ct, body)
	if r.Status != http.StatusConflict && r.Status != http.StatusNoContent {
		t.Errorf("certificado de B: %d %v", r.Status, r.Body)
	}
}
