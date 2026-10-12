//go:build integration

// Package integration prueba los adapters reales contra la base de dev (make test-integration): el consumidor de
// InvoiceIssued con el SQL de verdad, el inbox, la dead letter y las rutas internas del BFF.
package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"rdl/einvoice-api/internal/app"
	"rdl/einvoice-api/tests/testkit"
)

var env *testkit.Env

func TestMain(m *testing.M) {
	e, reason, err := testkit.Setup(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "integración:", err)
		os.Exit(1)
	}
	if reason != "" {
		fmt.Fprintln(os.Stderr, "AVISO: suite de integración saltada:", reason)
		os.Exit(0)
	}
	env = e
	code := m.Run()
	env.Close()
	os.Exit(code)
}

type docRow struct {
	ID                         uuid.UUID
	Status, TypeCode, Number   string
	Issuer, Receiver, Total    string
	Lines, Taxes, Methods, Hst int
}

func loadDocs(ctx context.Context, t *testing.T, tn testkit.Tenant, invoice uuid.UUID) []docRow {
	t.Helper()
	var out []docRow
	err := testkit.WithSession(ctx, env.Pool, tn.Org, uuid.Nil, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			select d.id, d.status, d.document_type_code, d.source_document_number, d.issuer_legal_name,
			       d.receiver_legal_name, d.total_amount::text,
			       (select count(*) from fiscal.electronic_document_lines l where l.electronic_document_id = d.id),
			       (select count(*) from fiscal.electronic_document_line_taxes x
			          join fiscal.electronic_document_lines l on l.id = x.electronic_document_line_id
			         where l.electronic_document_id = d.id),
			       (select count(*) from fiscal.electronic_document_payment_methods p where p.electronic_document_id = d.id),
			       (select count(*) from fiscal.electronic_document_status_history h where h.electronic_document_id = d.id)
			  from fiscal.electronic_documents d
			 where d.organization_id = $1 and d.source_document_id = $2`, tn.Org, invoice)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r docRow
			if err := rows.Scan(&r.ID, &r.Status, &r.TypeCode, &r.Number, &r.Issuer, &r.Receiver, &r.Total,
				&r.Lines, &r.Taxes, &r.Methods, &r.Hst); err != nil {
				return err
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Criterio 3: reprocesar un InvoiceIssued no crea un segundo documento fiscal.
func TestInvoiceIssuedCreatesExactlyOneDocument(t *testing.T) {
	ctx := context.Background()
	env.EnsureProfile(ctx, t, env.A)
	invoice, payload := testkit.IssueInvoice(t, env.A.Org)

	env.MustProcess(t, payload, app.OutcomeProcessed)
	env.MustProcess(t, payload, app.OutcomeDuplicate)

	docs := loadDocs(ctx, t, env.A, invoice)
	if len(docs) != 1 {
		t.Fatalf("se esperaba 1 documento, hay %d", len(docs))
	}
	d := docs[0]
	if d.Status != "processing" || d.TypeCode != "01" || d.Issuer == "" || d.Receiver == "" || d.Total != "11300.00000" ||
		d.Lines != 1 || d.Taxes != 1 || d.Methods != 1 || d.Hst != 1 {
		t.Errorf("documento = %+v", d)
	}

	// Otro evento (otro eventId) del mismo documento comercial tampoco crea un segundo: va a dead letter.
	again := testkit.Event(t, "invoice-issued.v1.json", env.A.Org, map[string]any{
		"invoiceId": invoice.String(), "invoiceNumber": d.Number,
	})
	env.MustProcess(t, again, app.OutcomeDeadLettered)
	if n := len(loadDocs(ctx, t, env.A, invoice)); n != 1 {
		t.Errorf("después del segundo evento hay %d documentos", n)
	}

	// La ruta interna del BFF lo ve en A; B no (404, no se distingue de inexistente).
	r := env.Call(t, http.MethodGet, "/internal/v1/electronic-documents/by-source/"+invoice.String(), env.A.OwnerToken(env))
	if r.Status != http.StatusOK || r.Str("status") != "processing" || r.Str("electronicDocumentId") != d.ID.String() {
		t.Errorf("by-source en A: %d %v", r.Status, r.Body)
	}
	r = env.Call(t, http.MethodGet, "/internal/v1/electronic-documents/by-source/"+invoice.String(), env.B.OwnerToken(env))
	if r.Status != http.StatusNotFound {
		t.Errorf("by-source desde B: %d %v", r.Status, r.Body)
	}

	// Por lote: A recibe su documento y omite el id que no existe; B no recibe nada.
	ids := map[string]any{"ids": []string{invoice.String(), uuid.NewString()}}
	r = env.CallBody(t, http.MethodPost, "/internal/v1/electronic-documents/by-source", env.A.OwnerToken(env), ids)
	items, _ := r.Body["items"].([]any)
	if r.Status != http.StatusOK || len(items) != 1 {
		t.Errorf("por lote en A: %d %v", r.Status, r.Body)
	}
	r = env.CallBody(t, http.MethodPost, "/internal/v1/electronic-documents/by-source", env.B.OwnerToken(env), ids)
	if items, _ := r.Body["items"].([]any); r.Status != http.StatusOK || len(items) != 0 {
		t.Errorf("por lote desde B: %d %v", r.Status, r.Body)
	}
	r = env.CallBody(t, http.MethodPost, "/internal/v1/electronic-documents/by-source", env.A.OwnerToken(env),
		map[string]any{"ids": []string{invoice.String(), invoice.String()}})
	if r.Status != http.StatusUnprocessableEntity {
		t.Errorf("ids repetidos: %d %v", r.Status, r.Body)
	}
}

// Un evento de B solo afecta a B: el organizationId del evento es el tenant de la transacción, y sin perfil fiscal
// en B no hay emisor, así que va a dead letter y no crea nada en ninguna organización.
func TestEventTenantIsItsOrganization(t *testing.T) {
	ctx := context.Background()
	var hasProfile bool
	err := testkit.WithSession(ctx, env.Pool, env.B.Org, uuid.Nil, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, "select exists (select 1 from fiscal.taxpayer_profiles where organization_id = $1)", env.B.Org).Scan(&hasProfile)
	})
	if err != nil {
		t.Fatal(err)
	}
	if hasProfile {
		t.Skip("B ya tiene perfil fiscal: este caso necesita una organización sin perfil")
	}
	invoice, payload := testkit.IssueInvoice(t, env.B.Org)
	env.MustProcess(t, payload, app.OutcomeDeadLettered)
	if n := len(loadDocs(ctx, t, env.B, invoice)) + len(loadDocs(ctx, t, env.A, invoice)); n != 0 {
		t.Errorf("un evento sin emisor creó %d documentos", n)
	}
}
