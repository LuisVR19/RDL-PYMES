//go:build integration

package isolation

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// Cada API revalida la organización contra su base; lo que esta suite comprueba es que, pasando por el
// gateway, esa revalidación sigue siendo la única que decide: el gateway no la debilita, no la sustituye y no
// cruza identidades entre peticiones.

func TestEachUserSeesOnlyItsOwnOrganization(t *testing.T) {
	requireEnv(t)
	for _, u := range []tenant{a, b} {
		r := call(t, http.MethodGet, "/portal/v1/organizations/current", u.Token, nil)
		expect(t, "GET organizations/current", http.StatusOK, r)
		if r.str("id") != u.OrgID {
			t.Fatalf("%s ve la organización %s, se esperaba %s", u.Email, r.str("id"), u.OrgID)
		}
	}
}

func TestMembershipsDoNotLeakTheOtherOrganization(t *testing.T) {
	requireEnv(t)
	r := call(t, http.MethodGet, "/portal/v1/me/memberships", a.Token, nil)
	expect(t, "GET me/memberships", http.StatusOK, r)
	orgs := r.ids("organizationId")
	if !slices.Contains(orgs, a.OrgID) {
		t.Fatalf("A no ve su propia organización en sus membresías: %v", orgs)
	}
	if slices.Contains(orgs, b.OrgID) {
		t.Fatal("A ve la organización de B entre sus membresías")
	}
}

// El cliente no puede elegir la organización: ni por query, ni por header. El gateway borra la query y no
// reenvía el header; si alguno llegara, Platform igual lo ignoraría. Aquí se prueban los dos a la vez.
func TestOrganizationFromTheClientIsIgnored(t *testing.T) {
	requireEnv(t)
	for _, q := range []string{"organizationId", "organization_id", "orgId", "org_id"} {
		r := call(t, http.MethodGet, "/portal/v1/organizations/current?"+q+"="+b.OrgID, a.Token, nil,
			"X-Organization-Id", b.OrgID)
		expect(t, "GET current con "+q, http.StatusOK, r)
		if r.str("id") != a.OrgID {
			t.Fatalf("con %s=%s, A terminó viendo %s", q, b.OrgID, r.str("id"))
		}
	}
}

func TestCannotSwitchIntoAForeignOrganization(t *testing.T) {
	requireEnv(t)
	r := call(t, http.MethodPut, "/portal/v1/me/active-organization", a.Token, map[string]string{"organizationId": b.OrgID})
	expect(t, "A activa la organización de B", http.StatusNotFound, r)
}

func TestForeignBranchIsNotFound(t *testing.T) {
	requireEnv(t)
	branch := createBranch(t, b)

	path := "/portal/v1/organizations/current/branches/" + branch
	expect(t, "A lee la sucursal de B", http.StatusNotFound, call(t, http.MethodGet, path, a.Token, nil))
	expect(t, "A edita la sucursal de B", http.StatusNotFound,
		call(t, http.MethodPatch, path, a.Token, map[string]any{"name": "Tomada por A"}))

	r := call(t, http.MethodGet, "/portal/v1/organizations/current/branches", a.Token, nil)
	expect(t, "A lista sus sucursales", http.StatusOK, r)
	if slices.Contains(r.ids("id"), branch) {
		t.Fatal("la sucursal de B aparece en el listado de A")
	}

	// Y B la sigue viendo intacta.
	r = call(t, http.MethodGet, path, b.Token, nil)
	expect(t, "B lee su sucursal", http.StatusOK, r)
	if strings.Contains(r.str("name"), "Tomada") {
		t.Fatal("la edición de A llegó a la sucursal de B")
	}
}

func TestForeignInvitationCannotBeRevoked(t *testing.T) {
	requireEnv(t)
	run := randomDigits(6)
	r := call(t, http.MethodPost, "/portal/v1/organizations/current/invitations", b.Token,
		map[string]string{"email": "gw-iso-inv-" + run + "@rdlpymes.invalid", "role": "biller"},
		"Idempotency-Key", "gw-iso-inv-"+uuid.NewString())
	expect(t, "B invita", http.StatusCreated, r)
	inv := r.str("id")

	expect(t, "A revoca la invitación de B", http.StatusNotFound,
		call(t, http.MethodDelete, "/portal/v1/organizations/current/invitations/"+inv, a.Token, nil))

	r = call(t, http.MethodGet, "/portal/v1/organizations/current/invitations", a.Token, nil)
	expect(t, "A lista sus invitaciones", http.StatusOK, r)
	if slices.Contains(r.ids("id"), inv) {
		t.Fatal("la invitación de B aparece en el listado de A")
	}
	r = call(t, http.MethodGet, "/portal/v1/organizations/current/invitations", b.Token, nil)
	expect(t, "B lista sus invitaciones", http.StatusOK, r)
	if !slices.Contains(r.ids("id"), inv) {
		t.Fatal("la invitación de B desapareció después del intento de A")
	}
}

// El gateway comparte clientes HTTP entre peticiones. Si alguna vez el token o la identidad quedaran en un
// estado compartido, dos usuarios intercalados verían datos cruzados: esto lo buscaría con carga concurrente.
func TestConcurrentUsersNeverMixIdentities(t *testing.T) {
	requireEnv(t)
	const perUser = 15
	var wg sync.WaitGroup
	errs := make(chan string, 4*perUser)
	for _, u := range []tenant{a, b} {
		for i := range perUser {
			wg.Add(1)
			go func() {
				defer wg.Done()
				path := "/portal/v1/organizations/current"
				field, want := "id", u.OrgID
				if i%2 == 1 {
					path, field, want = "/portal/v1/me", "email", u.Email
				}
				r := do(http.MethodGet, path, u.Token, nil)
				if r.Status != http.StatusOK {
					errs <- fmt.Sprintf("%s %s: status %d", u.Email, path, r.Status)
					return
				}
				if got := r.str(field); !strings.EqualFold(got, want) {
					errs <- fmt.Sprintf("%s %s: %s=%q, se esperaba %q", u.Email, path, field, got, want)
				}
			}()
		}
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

// Un token real con la firma alterada se corta en el gateway: la API ni se entera.
func TestTamperedTokenIsRejectedByTheGateway(t *testing.T) {
	requireEnv(t)
	tampered := a.Token[:len(a.Token)-4] + flip(a.Token[len(a.Token)-4:])
	r := call(t, http.MethodGet, "/portal/v1/organizations/current", tampered, nil)
	expect(t, "token alterado", http.StatusUnauthorized, r)
	if !strings.HasPrefix(r.problemType(), "urn:rdl:portal-gateway:problem:") {
		t.Fatalf("el 401 debería ser del gateway, llegó type %q", r.problemType())
	}
}

// --- Billing y la vista transversal ---

func TestForeignCustomerIsNotFound(t *testing.T) {
	requireBilling(t)
	customer := createCustomer(t, b)

	path := "/portal/v1/customers/" + customer
	expect(t, "A lee el cliente de B", http.StatusNotFound, call(t, http.MethodGet, path, a.Token, nil))
	expect(t, "A edita el cliente de B", http.StatusNotFound,
		call(t, http.MethodPatch, path, a.Token, map[string]any{"legalName": "Tomado por A"}))

	r := call(t, http.MethodGet, "/portal/v1/customers", a.Token, nil)
	expect(t, "A lista sus clientes", http.StatusOK, r)
	if slices.Contains(r.ids("id"), customer) {
		t.Fatal("el cliente de B aparece en el listado de A")
	}
}

// La vista transversal compone varias APIs: si la principal (Billing) dice 404 para A, la vista entera es 404,
// sin filtrar nada de las secundarias.
func TestForeignInvoiceAndItsOverviewAreNotFound(t *testing.T) {
	requireBilling(t)
	customer := createCustomer(t, b)
	r := call(t, http.MethodPost, "/portal/v1/invoices", b.Token, map[string]any{
		"documentType": "invoice", "customerId": customer, "saleConditionCode": "01", "currency": "CRC",
	}, "Idempotency-Key", "gw-iso-inv-"+uuid.NewString())
	expect(t, "B crea un borrador", http.StatusCreated, r)
	invoice := r.str("id")

	for _, p := range []string{"", "/history", "/overview"} {
		path := "/portal/v1/invoices/" + invoice + p
		expect(t, "A lee "+path, http.StatusNotFound, call(t, http.MethodGet, path, a.Token, nil))
	}
	expect(t, "A descarta el borrador de B", http.StatusNotFound,
		call(t, http.MethodDelete, "/portal/v1/invoices/"+invoice, a.Token, nil))

	r = call(t, http.MethodGet, "/portal/v1/invoices/"+invoice+"/overview", b.Token, nil)
	expect(t, "B lee su overview", http.StatusOK, r)

	// El listado compuesto (pantalla 12): la factura de B aparece para B y nunca para A.
	if listedInvoice(t, a, invoice) {
		t.Fatal("el borrador de B aparece en el listado de A")
	}
	if !listedInvoice(t, b, invoice) {
		t.Fatal("B no ve su borrador en su listado")
	}
}

// listedInvoice recorre el listado del gateway del usuario buscando la factura, página a página.
func listedInvoice(t *testing.T, u tenant, invoice string) bool {
	t.Helper()
	cursor := ""
	for range 50 {
		path := "/portal/v1/invoices?limit=100"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		r := call(t, http.MethodGet, path, u.Token, nil)
		expect(t, "listado de "+u.Email, http.StatusOK, r)
		items, _ := r.Body["items"].([]any)
		for _, it := range items {
			inv, _ := it.(map[string]any)["invoice"].(map[string]any)
			if inv["id"] == invoice {
				return true
			}
		}
		next, _ := r.Body["nextCursor"].(string)
		if next == "" {
			return false
		}
		cursor = next
	}
	t.Fatal("el listado no terminó en 50 páginas")
	return false
}

// --- datos de apoyo, siempre creados por el dueño y a través del gateway ---

func createBranch(t *testing.T, u tenant) string {
	t.Helper()
	code := "GW-" + randomDigits(4)
	r := call(t, http.MethodPost, "/portal/v1/organizations/current/branches", u.Token,
		map[string]string{"code": code, "name": "Sucursal de aislamiento " + code},
		"Idempotency-Key", "gw-iso-br-"+uuid.NewString())
	expect(t, "crear sucursal", http.StatusCreated, r)
	return r.str("id")
}

func createCustomer(t *testing.T, u tenant) string {
	t.Helper()
	run := randomDigits(6)
	r := call(t, http.MethodPost, "/portal/v1/customers", u.Token, map[string]any{
		"identification": map[string]string{"typeCode": "02", "number": "3101" + run},
		"legalName":      "Cliente aislamiento " + run + " S.A.",
		"email":          "gw-iso-c-" + run + "@rdlpymes.invalid",
	}, "Idempotency-Key", "gw-iso-c-"+uuid.NewString())
	expect(t, "crear cliente", http.StatusCreated, r)
	return r.str("id")
}

// flip cambia cada carácter base64url por otro distinto, para que la firma deje de coincidir.
func flip(s string) string {
	out := []byte(s)
	for i, c := range out {
		if c == 'A' {
			out[i] = 'B'
		} else {
			out[i] = 'A'
		}
	}
	return string(out)
}
