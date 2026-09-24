//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	cevents "bitbucket.org/rdl/contracts/pkg/events"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"rdl/billing-api/internal/adapters/postgres/db"
	"rdl/billing-api/internal/app"
	"rdl/billing-api/internal/domain/customer"
	"rdl/billing-api/internal/domain/invoice"
	"rdl/billing-api/internal/domain/money"
	"rdl/billing-api/internal/domain/numbering"
	"rdl/billing-api/internal/domain/product"
	"rdl/billing-api/internal/platform/config"
	"rdl/billing-api/pkg/correlation"
	"rdl/billing-api/pkg/tenancy"
)

// Pruebas del SQL de los repositorios contra la base dev con el login billing_api. Todo corre dentro de una
// transacción que se revierte al final: audit.audit_events es append-only y no se puede limpiar después.

var errRollback = errors.New("rollback de prueba")

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url, password := os.Getenv("TEST_DATABASE_URL"), os.Getenv("TEST_DB_PASSWORD")
	if url == "" {
		if err := config.LoadDotEnv("../../../.env"); err != nil {
			t.Fatal(err)
		}
		cfg, err := config.Load()
		if err != nil {
			t.Skipf("sin base de pruebas: %v", err)
		}
		url, password = cfg.DB.URL, cfg.DB.Password
	}
	p, err := NewPool(context.Background(), url, password, 2, 15*time.Second)
	if err != nil {
		t.Skipf("sin base de pruebas: %v", err)
	}
	t.Cleanup(p.Close)
	return p
}

// inRolledBackTx corre fn como la organización org y revierte todo al terminar.
func inRolledBackTx(t *testing.T, org uuid.UUID, fn func(ctx context.Context, tx pgx.Tx, r *tx) error) {
	t.Helper()
	ctx := context.Background()
	err := inTx(ctx, testPool(t), pgx.TxOptions{}, session{organizationID: org, userID: uuid.New()},
		func(q *db.Queries, ptx pgx.Tx) error {
			if err := fn(ctx, ptx, &tx{q: q}); err != nil {
				return err
			}
			return errRollback
		})
	if !errors.Is(err, errRollback) {
		t.Fatal(err)
	}
}

// inSavepoint corre una sentencia que se espera que falle sin abortar la transacción de la prueba.
func inSavepoint(ctx context.Context, ptx pgx.Tx, fn func(r *tx) error) error {
	sp, err := ptx.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = sp.Rollback(ctx) }()
	return fn(&tx{q: db.New(sp)})
}

func TestCustomersSQL(t *testing.T) {
	org := uuid.New()
	user := uuid.New()
	inRolledBackTx(t, org, func(ctx context.Context, ptx pgx.Tx, r *tx) error {
		in, err := customer.New(org, user, customer.NewInput{
			IdentificationTypeCode: "01", IdentificationNumber: "112340567", LegalName: "Ferretería 50% Descuento",
			Email: "ventas@example.com",
		})
		if err != nil {
			return err
		}
		c, err := r.Customers().Create(ctx, in)
		if err != nil {
			return fmt.Errorf("create: %w", err)
		}
		if c.ID == uuid.Nil || c.CreatedByUserID != user || !c.IsActive || c.TradeName != "" || c.CreatedAt.IsZero() {
			return fmt.Errorf("creado = %+v", c)
		}

		dupErr := inSavepoint(ctx, ptx, func(r *tx) error { _, err := r.Customers().Create(ctx, in); return err })
		if !errors.Is(dupErr, app.ErrCustomerIdentificationTaken) {
			return fmt.Errorf("duplicado: err = %w", dupErr)
		}

		// La búsqueda trata % como texto, no como comodín.
		for search, want := range map[string]int{"50%": 1, "%": 1, "60%": 0, "1123": 1, "ferre": 1} {
			items, err := r.Customers().List(ctx, org, app.CustomerQuery{Search: search, Limit: 10})
			if err != nil || len(items) != want {
				return fmt.Errorf("búsqueda %q: %d resultados, se esperaban %d (err %w)", search, len(items), want, err)
			}
		}
		items, err := r.Customers().List(ctx, org, app.CustomerQuery{Search: "_", Limit: 10})
		if err != nil || len(items) != 0 {
			return fmt.Errorf("búsqueda _ debe ser literal: %d resultados", len(items))
		}

		c.LegalName, c.Email, c.IsActive = "Ferretería Central", "", false
		upd, err := r.Customers().Update(ctx, c)
		if err != nil || upd.LegalName != "Ferretería Central" || upd.Email != "" || upd.IsActive {
			return fmt.Errorf("update: %+v %w", upd, err)
		}
		if _, err := r.Customers().GetForUpdate(ctx, org, c.ID); err != nil {
			return fmt.Errorf("get for update: %w", err)
		}
		active := true
		if items, _ := r.Customers().List(ctx, org, app.CustomerQuery{Active: &active, Limit: 10}); len(items) != 0 {
			return errors.New("filtro active=true devolvió un cliente inactivo")
		}

		// Con la sesión en otra organización, el cliente no existe.
		if err := setSession(ctx, ptx, session{organizationID: uuid.New(), userID: user}); err != nil {
			return err
		}
		if _, err := r.Customers().Get(ctx, org, c.ID); !errors.Is(err, app.ErrNotFound) {
			return fmt.Errorf("con otra sesión: err = %w, se esperaba ErrNotFound", err)
		}
		return nil
	})
}

func TestIdempotencyAndAuditSQL(t *testing.T) {
	org := uuid.New()
	hashA, hashB := strings.Repeat("a", 64), strings.Repeat("b", 64) // shared.sha256_hex
	inRolledBackTx(t, org, func(ctx context.Context, _ pgx.Tx, r *tx) error {
		prev, err := r.Idempotency().Claim(ctx, org, "k-1", hashA, time.Hour)
		if err != nil || prev != nil {
			return fmt.Errorf("primera reserva: prev=%v err=%w", prev, err)
		}
		id := uuid.NewString()
		if err := r.Idempotency().Complete(ctx, org, "k-1", app.IdempotencyRecord{
			RequestHash: hashA, Status: 201, Result: map[string]string{"customerId": id},
		}); err != nil {
			return err
		}
		prev, err = r.Idempotency().Claim(ctx, org, "k-1", hashB, time.Hour)
		if err != nil || prev == nil || prev.RequestHash != hashA || prev.Status != 201 || prev.Result["customerId"] != id {
			return fmt.Errorf("segunda reserva: prev=%+v err=%w", prev, err)
		}

		// La política de audit_events exige service = 'billing' (shared.current_service() de billing_app).
		return r.Audit().Record(ctx, app.AuditEvent{
			OrganizationID: org, ActorUserID: uuid.New(), Action: "customer.created", EntityType: "customer",
			EntityID: uuid.New(), After: map[string]any{"legalName": "X"},
		})
	})
}

func TestProductsSQL(t *testing.T) {
	org := uuid.New()
	inRolledBackTx(t, org, func(ctx context.Context, ptx pgx.Tx, r *tx) error {
		in, err := product.New(org, product.NewInput{
			Code: "SERV-01", Description: "Consultoría 100%", CabysCode: "8314100000100", UnitOfMeasureCode: "Sp",
			UnitPrice: money.MustAmountForTest("9999999999999.99999"), Currency: money.MustCurrencyForTest("USD"),
			IsService: true, Taxes: []product.Tax{{TypeCode: "02", RateCode: "01"}, {TypeCode: "01", RateCode: "08"}},
		})
		if err != nil {
			return err
		}
		p, err := r.Products().Create(ctx, in)
		if err != nil {
			return fmt.Errorf("create: %w", err)
		}
		// El máximo de numeric(18,5) ida y vuelta sin perder un dígito.
		if p.UnitPrice.String() != "9999999999999.99999" || p.Currency.String() != "USD" || len(p.Taxes) != 2 {
			return fmt.Errorf("creado = %+v", p)
		}
		got, err := r.Products().Get(ctx, org, p.ID)
		if err != nil || !got.Equal(p) || got.Taxes[0].TypeCode != "01" {
			return fmt.Errorf("get: %+v (err %w)", got, err)
		}

		dup := in
		dupErr := inSavepoint(ctx, ptx, func(r *tx) error { _, err := r.Products().Create(ctx, dup); return err })
		if !errors.Is(dupErr, app.ErrProductCodeTaken) {
			return fmt.Errorf("código duplicado: err = %w", dupErr)
		}

		// Reemplazar impuestos y precio; el precio se guarda con 5 decimales y vuelve en forma canónica.
		p.UnitPrice, p.Taxes = money.MustAmountForTest("1300.50000"), []product.Tax{{TypeCode: "01", RateCode: "04"}}
		upd, err := r.Products().Update(ctx, p)
		if err != nil || upd.UnitPrice.String() != "1300.5" || len(upd.Taxes) != 1 || upd.Taxes[0].RateCode != "04" {
			return fmt.Errorf("update: %+v (err %w)", upd, err)
		}
		again, _ := r.Products().Get(ctx, org, p.ID)
		if len(again.Taxes) != 1 || again.UnitPrice.String() != "1300.5" {
			return fmt.Errorf("después del update: %+v", again)
		}

		// Segundo producto: el listado trae los impuestos de cada uno con una sola consulta extra.
		other, _ := product.New(org, product.NewInput{Code: "PROD-02", Description: "Tornillo", CabysCode: "4299000000000",
			UnitOfMeasureCode: "Unid", UnitPrice: money.MustAmountForTest("0.5"), Currency: money.MustCurrencyForTest("CRC")})
		if _, err := r.Products().Create(ctx, other); err != nil {
			return err
		}
		// En una misma transacción now() no cambia: ambos tienen el mismo created_at y el orden lo desempata el id.
		items, err := r.Products().List(ctx, org, app.ProductQuery{Limit: 10})
		if err != nil || len(items) != 2 {
			return fmt.Errorf("list: %+v (err %w)", items, err)
		}
		byCode := map[string]product.Product{items[0].Code: items[0], items[1].Code: items[1]}
		if len(byCode["SERV-01"].Taxes) != 1 || byCode["PROD-02"].Taxes == nil || len(byCode["PROD-02"].Taxes) != 0 {
			return fmt.Errorf("impuestos en el listado: %+v", items)
		}
		if found, _ := r.Products().List(ctx, org, app.ProductQuery{Search: "100%", Limit: 10}); len(found) != 1 {
			return fmt.Errorf("búsqueda por descripción con %%: %d", len(found))
		}

		if err := setSession(ctx, ptx, session{organizationID: uuid.New(), userID: uuid.New()}); err != nil {
			return err
		}
		if _, err := r.Products().Get(ctx, org, p.ID); !errors.Is(err, app.ErrNotFound) {
			return fmt.Errorf("con otra sesión: err = %w, se esperaba ErrNotFound", err)
		}
		return nil
	})
}

// seedDraft crea un cliente, un producto y un borrador con una línea dentro de la transacción de la prueba.
func seedDraft(ctx context.Context, t *testing.T, r *tx, org, user uuid.UUID) invoice.Invoice {
	t.Helper()
	c, err := customer.New(org, user, customer.NewInput{IdentificationTypeCode: "01", IdentificationNumber: "1", LegalName: "Ana"})
	if err != nil {
		t.Fatal(err)
	}
	if c, err = r.Customers().Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	p, _ := product.New(org, product.NewInput{Code: "P", Description: "Tornillo", CabysCode: "4299000000000",
		UnitOfMeasureCode: "Unid", UnitPrice: money.MustAmountForTest("0.33333"), Currency: money.MustCurrencyForTest("CRC")})
	if p, err = r.Products().Create(ctx, p); err != nil {
		t.Fatal(err)
	}
	inv, err := invoice.NewDraft(org, user, invoice.Header{DocumentType: invoice.TypeInvoice, CustomerID: c.ID,
		SaleConditionCode: "01", Currency: money.MustCurrencyForTest("CRC"), ExchangeRate: money.MustExchangeRateForTest("1")})
	if err != nil {
		t.Fatal(err)
	}
	inv, err = inv.ReplaceLines([]invoice.LineDraft{{ProductID: &p.ID, ProductCode: p.Code, CabysCode: p.CabysCode,
		Description: p.Description, UnitOfMeasureCode: p.UnitOfMeasureCode, Quantity: money.MustQuantityForTest("3"),
		UnitPrice: p.UnitPrice, Discount: money.MustAmountForTest("0.00001"), DiscountReason: "redondeo",
		Taxes: []invoice.TaxDraft{{TypeCode: "01", RateCode: "08", Rate: money.MustPercentageForTest("13")}}}})
	if err != nil {
		t.Fatal(err)
	}
	if inv, err = r.Invoices().Create(ctx, inv); err != nil {
		t.Fatal(err)
	}
	return inv
}

func TestInvoicesSQL(t *testing.T) {
	org, user := uuid.New(), uuid.New()
	inRolledBackTx(t, org, func(ctx context.Context, ptx pgx.Tx, r *tx) error {
		inv := seedDraft(ctx, t, r, org, user)
		got, err := r.Invoices().Get(ctx, org, inv.ID)
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}
		l := got.Lines[0]
		// Lo guardado vuelve idéntico: montos canónicos, snapshot del producto y el impuesto con su tasa.
		if got.Status != invoice.StatusDraft || got.Number != "" || got.DueDate != "" || got.IssuedAt != nil ||
			got.Totals.Subtotal.String() != "0.99998" || got.Totals.Tax.String() != "0.13" || got.Totals.Total.String() != "1.12998" ||
			l.Quantity.String() != "3" || l.UnitPrice.String() != "0.33333" || l.DiscountReason != "redondeo" ||
			len(l.Taxes) != 1 || l.Taxes[0].Rate.String() != "13" || l.Taxes[0].TaxableBase.String() != "0.99998" {
			return fmt.Errorf("leído = %+v / línea %+v", got, l)
		}

		// Reemplazar líneas por ninguna deja el borrador en cero.
		empty, _ := got.ReplaceLines(nil)
		saved, err := r.Invoices().SaveDraft(ctx, empty)
		if err != nil {
			return fmt.Errorf("save: %w", err)
		}
		again, _ := r.Invoices().Get(ctx, org, inv.ID)
		if len(again.Lines) != 0 || again.Totals.Total.String() != "0" || !saved.UpdatedAt.Equal(again.UpdatedAt) {
			return fmt.Errorf("después de vaciar: %+v", again)
		}

		status := app.InvoiceQuery{Status: invoice.StatusDraft, Limit: 10}
		if items, err := r.Invoices().List(ctx, org, status); err != nil || len(items) != 1 {
			return fmt.Errorf("list drafts: %d (err %w)", len(items), err)
		}
		// Filtro por fecha de emisión: un borrador no tiene fecha y no aparece.
		dated := app.InvoiceQuery{IssuedFrom: "2026-01-01", Timezone: "America/Costa_Rica", Limit: 10}
		if items, err := r.Invoices().List(ctx, org, dated); err != nil || len(items) != 0 {
			return fmt.Errorf("list por fecha: %d (err %w)", len(items), err)
		}
		if h, err := r.Invoices().History(ctx, org, inv.ID); err != nil || len(h) != 0 {
			return fmt.Errorf("historial: %v (err %w)", h, err)
		}

		if err := r.Invoices().DeleteDraft(ctx, org, inv.ID); err != nil {
			return fmt.Errorf("delete: %w", err)
		}
		if _, err := r.Invoices().Get(ctx, org, inv.ID); !errors.Is(err, app.ErrNotFound) {
			return fmt.Errorf("después de descartar: err = %w", err)
		}
		return nil
	})
}

// TestIssuedInvoiceIsImmutableInTheDatabase: aunque la app se equivoque, la base no deja tocar lo emitido.
func TestIssuedInvoiceIsImmutableInTheDatabase(t *testing.T) {
	org, user := uuid.New(), uuid.New()
	inRolledBackTx(t, org, func(ctx context.Context, ptx pgx.Tx, r *tx) error {
		inv := seedDraft(ctx, t, r, org, user)
		// Emisión a mano (el caso de uso llega en el incremento 8), con lo mínimo que exige invoices_issued_ck.
		if _, err := ptx.Exec(ctx, `update billing.invoices
			   set status = 'issued', number = 'T-1', issued_at = now(), issued_by_user_id = $3,
			       customer_identification_type_code = '01', customer_identification_number = '1', customer_legal_name = 'Ana'
			 where organization_id = $1 and id = $2`, org, inv.ID, user); err != nil {
			return fmt.Errorf("emitiendo a mano: %w", err)
		}
		issued, err := r.Invoices().Get(ctx, org, inv.ID)
		if err != nil {
			return err
		}
		empty := issued
		empty.Status = invoice.StatusDraft // simula un error de la app que "olvida" el estado
		empty.Lines = nil
		errSave := inSavepoint(ctx, ptx, func(r *tx) error { _, err := r.Invoices().SaveDraft(ctx, empty); return err })
		if !errors.Is(errSave, invoice.ErrNotDraft) {
			return fmt.Errorf("save sobre emitido: err = %w", errSave)
		}
		errDel := inSavepoint(ctx, ptx, func(r *tx) error { return r.Invoices().DeleteDraft(ctx, org, inv.ID) })
		if !errors.Is(errDel, invoice.ErrNotDraft) {
			return fmt.Errorf("delete sobre emitido: err = %w", errDel)
		}
		// Las líneas las protege guard_draft_children (restrict_violation → ErrNotDraft).
		errLines := inSavepoint(ctx, ptx, func(r *tx) error {
			return guardErr("x", r.q.DeleteInvoiceLines(ctx, db.DeleteInvoiceLinesParams{OrganizationID: org, InvoiceID: inv.ID}))
		})
		if !errors.Is(errLines, invoice.ErrNotDraft) {
			return fmt.Errorf("borrar líneas de emitido: err = %w", errLines)
		}
		after, _ := r.Invoices().Get(ctx, org, inv.ID)
		if after.Status != invoice.StatusIssued || len(after.Lines) != 1 || after.Totals.Total != issued.Totals.Total {
			return fmt.Errorf("el emitido cambió: %+v", after)
		}
		return nil
	})
}

func TestCatalogSQL(t *testing.T) {
	org := uuid.New()
	inRolledBackTx(t, org, func(ctx context.Context, _ pgx.Tx, r *tx) error {
		// fiscal.tax_rates está vacío en dev: un código desconocido no aparece (nunca se inventa una tasa).
		rates, err := r.Catalog().TaxRates(ctx, []string{"08", "no-existe"})
		if err != nil || len(rates) != 0 && rates["no-existe"].String() != "" {
			return fmt.Errorf("tarifas: %v (err %w)", rates, err)
		}
		if _, err := r.Catalog().Branch(ctx, org, uuid.New()); !errors.Is(err, app.ErrNotFound) {
			return fmt.Errorf("sucursal inexistente: err = %w", err)
		}
		return nil
	})
	// Con una organización real de prueba (TEST_MEMBER_ORG), sus datos de core son visibles bajo su propia sesión.
	if real := os.Getenv("TEST_MEMBER_ORG"); real != "" {
		orgID := uuid.MustParse(real)
		inRolledBackTx(t, orgID, func(ctx context.Context, _ pgx.Tx, r *tx) error {
			s, err := r.Catalog().Organization(ctx, orgID)
			if err != nil || s.LocalCurrency.String() == "" || s.Timezone == "" {
				return fmt.Errorf("organización: %+v (err %w)", s, err)
			}
			if _, err := r.Catalog().Organization(ctx, uuid.New()); err == nil {
				return errors.New("otra organización no debe ser visible")
			}
			return nil
		})
	}
}

// TestSequencesSQL necesita las migraciones 00002 (last_assigned_number) y 00003 (FK de sucursal).
func TestSequencesSQL(t *testing.T) {
	org := uuid.New()
	inRolledBackTx(t, org, func(ctx context.Context, ptx pgx.Tx, r *tx) error {
		repo := r.Sequences()
		if err := repo.Lock(ctx, org, "invoice"); err != nil {
			return fmt.Errorf("lock: %w", err)
		}
		scope := numbering.Scope{DocumentType: "invoice"}
		if _, found, err := repo.GetForUpdate(ctx, org, scope); err != nil || found {
			return fmt.Errorf("antes de crear: found=%v err=%w", found, err)
		}
		s, _ := numbering.New(org, scope).Configure("FAC-", 41, nil)
		saved, err := repo.Save(ctx, s)
		if err != nil || saved.ID == uuid.Nil || saved.Used() || saved.NextNumber != 41 {
			return fmt.Errorf("insert: %+v (err %w)", saved, err)
		}
		number, used := saved.Assign()
		if number != "FAC-00000041" {
			return fmt.Errorf("número = %s", number)
		}
		after, err := repo.Save(ctx, used)
		if err != nil || !after.Used() || *after.LastAssigned != 41 || after.NextNumber != 42 {
			return fmt.Errorf("update: %+v (err %w)", after, err)
		}
		locked, found, err := repo.GetForUpdate(ctx, org, scope)
		if err != nil || !found || locked.ID != saved.ID || !locked.Used() {
			return fmt.Errorf("get for update: %+v found=%v (err %w)", locked, found, err)
		}
		// La base rechaza una marca de uso incoherente (último asignado ≥ próximo).
		bad := after
		bad.NextNumber = 41
		if err := inSavepoint(ctx, ptx, func(r *tx) error { _, err := r.Sequences().Save(ctx, bad); return err }); err == nil {
			return errors.New("la base aceptó last_assigned_number ≥ next_number")
		}
		// Una sucursal que no es de la organización: la FK compuesta de 00003 la rechaza (si ya está aplicada).
		var hasFK bool
		if err := ptx.QueryRow(ctx, `select exists (select 1 from pg_constraint where conname = 'document_sequences_branch_fk')`).Scan(&hasFK); err != nil {
			return err
		}
		if hasFK {
			ghost := uuid.New()
			branchSeq := numbering.New(org, numbering.Scope{DocumentType: "invoice", BranchID: &ghost})
			if err := inSavepoint(ctx, ptx, func(r *tx) error { _, err := r.Sequences().Save(ctx, branchSeq); return err }); err == nil {
				return errors.New("la base aceptó una sucursal inexistente")
			}
		} else {
			t.Log("migración 00003 (FK de sucursal) sin aplicar: se omite esa verificación")
		}
		list, err := repo.List(ctx, org)
		if err != nil || len(list) != 1 {
			return fmt.Errorf("list: %d (err %w)", len(list), err)
		}
		return nil
	})
}

// TestIssueEndToEndSQL: la emisión completa contra la base con billing_api. Necesita una organización real de prueba
// (TEST_MEMBER_ORG) porque lee su zona horaria de core.organizations.
func TestIssueEndToEndSQL(t *testing.T) {
	orgEnv := os.Getenv("TEST_MEMBER_ORG")
	if orgEnv == "" {
		t.Skip("defina TEST_MEMBER_ORG (organización de prueba de Platform) para probar la emisión completa")
	}
	org := uuid.MustParse(orgEnv)
	inRolledBackTx(t, org, func(ctx context.Context, ptx pgx.Tx, r *tx) error {
		tm := SavepointTxManager{Outer: ptx}
		owner := tenancy.NewContext(uuid.New(), "sub", org, []string{"owner"})
		ctx = correlation.WithID(ctx, uuid.New())

		c, err := app.NewCreateCustomer(tm).Execute(ctx, owner, uuid.NewString(), customer.NewInput{
			IdentificationTypeCode: "02", IdentificationNumber: "3101" + uuid.NewString()[:6], LegalName: "Cliente de prueba S.A.",
			Email: "facturas@cliente.example",
		})
		if err != nil {
			return fmt.Errorf("cliente: %w", err)
		}
		// Sin impuestos: fiscal.tax_rates está vacío en dev.
		p, err := app.NewCreateProduct(tm).Execute(ctx, owner, uuid.NewString(), product.NewInput{
			Code: "E2E-" + uuid.NewString()[:8], Description: "Servicio de prueba", CabysCode: "8314100000100",
			UnitOfMeasureCode: "Sp", UnitPrice: money.MustAmountForTest("0.33333"), Currency: money.MustCurrencyForTest("CRC"), IsService: true,
		})
		if err != nil {
			return fmt.Errorf("producto: %w", err)
		}
		if _, err := app.NewConfigureSequence(tm).Execute(ctx, owner, app.SequenceInput{
			DocumentType: invoice.TypeInvoice, Prefix: "E2E-", NextNumber: 7,
		}); err != nil {
			return fmt.Errorf("secuencia: %w", err)
		}
		d, err := app.NewCreateInvoiceDraft(tm).Execute(ctx, owner, uuid.NewString(), app.HeaderInput{
			DocumentType: invoice.TypeInvoice, CustomerID: c.Customer.ID, SaleConditionCode: "01", Currency: money.MustCurrencyForTest("CRC"),
		}, []app.LineRequest{{ProductID: p.Product.ID, Quantity: money.MustQuantityForTest("3")}})
		if err != nil {
			return fmt.Errorf("borrador: %w", err)
		}

		key := uuid.NewString()
		res, err := app.NewIssueInvoice(tm).Execute(ctx, owner, key, d.Invoice.ID)
		if err != nil {
			return fmt.Errorf("emisión: %w", err)
		}
		inv := res.Invoice
		if inv.Number != "E2E-00000007" || inv.Status != invoice.StatusIssued || inv.Totals.Total.String() != "0.99999" {
			return fmt.Errorf("emitido = %+v", inv)
		}

		// Todo quedó escrito en la misma transacción: factura, historial, secuencia y outbox.
		stored, err := r.Invoices().Get(ctx, org, inv.ID)
		if err != nil || stored.Status != invoice.StatusIssued || stored.Customer.LegalName != "Cliente de prueba S.A." || stored.DueDate == "" {
			return fmt.Errorf("guardado = %+v (err %w)", stored, err)
		}
		if h, _ := r.Invoices().History(ctx, org, inv.ID); len(h) != 1 || h[0].To != invoice.StatusIssued {
			return fmt.Errorf("historial = %+v", h)
		}
		seq, _, _ := r.Sequences().GetForUpdate(ctx, org, numbering.Scope{DocumentType: "invoice"})
		if seq.NextNumber != 8 || *seq.LastAssigned != 7 {
			return fmt.Errorf("secuencia = %+v", seq)
		}
		var payload string
		var eventType, source string
		if err := ptx.QueryRow(ctx, `select event_type, source_service, payload::text from integration.outbox_messages
		                             where aggregate_id = $1`, inv.ID).Scan(&eventType, &source, &payload); err != nil {
			return fmt.Errorf("outbox: %w", err)
		}
		if eventType != "InvoiceIssued" || source != "billing" {
			return fmt.Errorf("outbox = %s/%s", eventType, source)
		}
		v, _ := cevents.DefaultValidator()
		if err := v.Validate(cevents.InvoiceIssuedSchemaV1, []byte(payload)); err != nil {
			return fmt.Errorf("el payload guardado no valida: %w", err)
		}

		// Reintento con la misma clave: mismo resultado, sin segundo evento.
		again, err := app.NewIssueInvoice(tm).Execute(ctx, owner, key, d.Invoice.ID)
		if err != nil || !again.Replayed || again.Invoice.Number != inv.Number {
			return fmt.Errorf("reintento: %+v (err %w)", again, err)
		}
		var events int
		_ = ptx.QueryRow(ctx, `select count(*) from integration.outbox_messages where aggregate_id = $1`, inv.ID).Scan(&events)
		if events != 1 {
			return fmt.Errorf("eventos en el outbox = %d", events)
		}

		// Criterio 5: cambiar cliente y producto no altera la factura emitida ni su evento.
		name := "Otro nombre"
		if _, err := app.NewUpdateCustomer(tm).Execute(ctx, owner, c.Customer.ID, customer.Patch{LegalName: &name}); err != nil {
			return err
		}
		price := money.MustAmountForTest("12345")
		if _, err := app.NewUpdateProduct(tm).Execute(ctx, owner, p.Product.ID, product.Patch{UnitPrice: &price}); err != nil {
			return err
		}
		after, _ := r.Invoices().Get(ctx, org, inv.ID)
		var payloadAfter string
		_ = ptx.QueryRow(ctx, `select payload::text from integration.outbox_messages where aggregate_id = $1`, inv.ID).Scan(&payloadAfter)
		if after.Customer.LegalName != "Cliente de prueba S.A." || after.Lines[0].UnitPrice.String() != "0.33333" || payloadAfter != payload {
			return fmt.Errorf("lo emitido cambió: %+v", after)
		}

		// Emitir otra vez con otra clave: 409.
		if _, err := app.NewIssueInvoice(tm).Execute(ctx, owner, uuid.NewString(), d.Invoice.ID); !errors.Is(err, invoice.ErrNotDraft) {
			return fmt.Errorf("segunda emisión: err = %w", err)
		}
		return nil
	})
}
