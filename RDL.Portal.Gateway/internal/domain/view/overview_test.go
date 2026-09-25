package view

import "testing"

func TestPartKeepsValueOnlyWhenAvailable(t *testing.T) {
	status := &FiscalStatus{ElectronicDocumentID: "doc-1", Status: "accepted"}

	if p := NewFiscalPart(Available, status); p.Status == nil || p.Availability != Available {
		t.Errorf("disponible con dato: %+v", p)
	}
	// Un dato que llegó pero cuya parte no está disponible no se publica: el portal mostraría información vieja
	// o incompleta como si fuera buena.
	for _, a := range []Availability{Unavailable, Absent} {
		if p := NewFiscalPart(a, status); p.Status != nil {
			t.Errorf("%s: no debe publicar el dato, %+v", a, p)
		}
	}
}

// Estado imposible: la API contestó 200 pero sin cuerpo útil. Para el portal es lo mismo que no haber contestado.
func TestAvailableWithoutValueDegradesToUnavailable(t *testing.T) {
	if p := NewFiscalPart(Available, nil); p.Availability != Unavailable {
		t.Errorf("fiscal: %s, want %s", p.Availability, Unavailable)
	}
	if p := NewBalancePart(Available, nil); p.Availability != Unavailable {
		t.Errorf("saldo: %s, want %s", p.Availability, Unavailable)
	}
}

func TestDegraded(t *testing.T) {
	balance := &Balance{ReceivableID: "r-1", BalanceAmount: "63000.00"}
	status := &FiscalStatus{ElectronicDocumentID: "doc-1", Status: "accepted"}

	cases := []struct {
		name     string
		overview InvoiceOverview
		want     bool
	}{
		{"todo disponible", InvoiceOverview{
			Fiscal:     NewFiscalPart(Available, status),
			Receivable: NewBalancePart(Available, balance),
		}, false},
		// Un borrador no tiene documento electrónico ni cuenta por cobrar: eso no es una degradación.
		{"borrador sin partes", InvoiceOverview{
			Fiscal:     NewFiscalPart(Absent, nil),
			Receivable: NewBalancePart(Absent, nil),
		}, false},
		{"fiscal caído", InvoiceOverview{
			Fiscal:     NewFiscalPart(Unavailable, nil),
			Receivable: NewBalancePart(Available, balance),
		}, true},
		{"receivables caído", InvoiceOverview{
			Fiscal:     NewFiscalPart(Available, status),
			Receivable: NewBalancePart(Unavailable, nil),
		}, true},
	}
	for _, c := range cases {
		if got := c.overview.Degraded(); got != c.want {
			t.Errorf("%s: Degraded()=%v, want %v", c.name, got, c.want)
		}
	}
}
