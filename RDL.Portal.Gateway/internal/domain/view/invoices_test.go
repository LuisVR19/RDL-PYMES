package view

import "testing"

func rowsOf(ids ...string) InvoiceRows {
	var r InvoiceRows
	for _, id := range ids {
		r.Rows = append(r.Rows, InvoiceRow{ID: id, Total: "1.00000"})
	}
	return r
}

func TestJoinInvoicePageMarksEachRow(t *testing.T) {
	rows := rowsOf("a", "b")
	rows.NextCursor = "c2"
	page := JoinInvoicePage(rows,
		Batch[FiscalStatus]{Available: true, Found: map[string]FiscalStatus{"a": {ElectronicDocumentID: "d", Status: "accepted"}}},
		Batch[Balance]{Available: false},
	)
	if len(page.Items) != 2 || page.NextCursor != "c2" {
		t.Fatalf("página = %+v", page)
	}
	a, b := page.Items[0], page.Items[1]
	if a.Invoice.ID != "a" || b.Invoice.ID != "b" {
		t.Fatal("el orden tiene que ser el de Billing")
	}
	if a.Fiscal.Availability != Available || a.Fiscal.Status.Status != "accepted" {
		t.Errorf("a.fiscal = %+v", a.Fiscal)
	}
	// El lote respondió y no trae a b: b no tiene documento electrónico, no es una falla.
	if b.Fiscal.Availability != Absent || b.Fiscal.Status != nil {
		t.Errorf("b.fiscal = %+v", b.Fiscal)
	}
	for _, it := range page.Items {
		if it.Receivable.Availability != Unavailable || it.Receivable.Balance != nil {
			t.Errorf("saldo de %s = %+v: el lote no se pudo consultar", it.Invoice.ID, it.Receivable)
		}
	}
	if !page.Degraded() {
		t.Error("una parte unavailable degrada la página")
	}
}

func TestJoinInvoicePageEmpty(t *testing.T) {
	page := JoinInvoicePage(InvoiceRows{}, Batch[FiscalStatus]{Available: true}, Batch[Balance]{Available: true})
	if page.Items == nil || len(page.Items) != 0 || page.Degraded() {
		t.Fatalf("página vacía = %+v", page)
	}
}
