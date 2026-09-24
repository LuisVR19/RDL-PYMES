package invoice

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func issuable(t *testing.T) Invoice {
	t.Helper()
	inv, _ := NewDraft(uuid.New(), uuid.New(), header())
	inv, err := inv.ReplaceLines([]LineDraft{draftLine("2", "1500", "250", "Promo", "13")})
	if err != nil {
		t.Fatal(err)
	}
	inv.ID = uuid.New()
	return inv
}

func issueInput() IssueInput {
	return IssueInput{
		Number: "FAC-00000001", IssuedAt: time.Date(2026, 9, 25, 5, 30, 0, 0, time.UTC), IssueDate: "2026-09-24",
		Customer: CustomerSnapshot{IdentificationTypeCode: "02", IdentificationNumber: "3101123456", LegalName: "Cliente S.A."},
		IssuedBy: uuid.New(),
	}
}

func TestIssue(t *testing.T) {
	inv := issuable(t)
	in := issueInput()
	out, err := inv.Issue(in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != StatusIssued || out.Number != "FAC-00000001" || !out.IssuedAt.Equal(in.IssuedAt) ||
		*out.IssuedByUserID != in.IssuedBy || out.Customer.LegalName != "Cliente S.A." || out.DueDate != "2026-09-24" {
		t.Fatalf("emitido = %+v", out)
	}
	// Las líneas y los montos quedan tal cual estaban en el borrador.
	if out.Totals != inv.Totals || out.Lines[0].Total != inv.Lines[0].Total {
		t.Fatal("la emisión no recalcula")
	}
	if inv.Status != StatusDraft {
		t.Fatal("Issue no modifica el original")
	}
}

func TestIssueDueDateWithCredit(t *testing.T) {
	inv := issuable(t)
	days := 30
	inv.CreditTermDays = &days
	out, err := inv.Issue(issueInput())
	if err != nil || out.DueDate != "2026-10-24" {
		t.Fatalf("vencimiento = %q err=%v", out.DueDate, err)
	}
}

func TestDueDate(t *testing.T) {
	zero, thirty := 0, 30
	for _, c := range []struct {
		issue string
		days  *int
		want  string
	}{
		{"2026-09-24", nil, "2026-09-24"},
		{"2026-09-24", &zero, "2026-09-24"},
		{"2026-01-31", &thirty, "2026-03-02"},
		{"2028-02-15", &thirty, "2028-03-16"}, // año bisiesto
	} {
		if got, err := DueDate(c.issue, c.days); err != nil || got != c.want {
			t.Errorf("DueDate(%s, %v) = %s (err %v), se esperaba %s", c.issue, c.days, got, err, c.want)
		}
	}
}

func TestIssueRules(t *testing.T) {
	cases := map[string]struct {
		mut  func(*Invoice, *IssueInput)
		want error
	}{
		"ya emitido":             {func(i *Invoice, _ *IssueInput) { i.Status = StatusIssued }, ErrNotDraft},
		"anulado":                {func(i *Invoice, _ *IssueInput) { i.Status = StatusCancelled }, ErrNotDraft},
		"sin líneas":             {func(i *Invoice, _ *IssueInput) { i.Lines = nil }, ErrWithoutLines},
		"totales que no cuadran": {func(i *Invoice, _ *IssueInput) { i.Totals.Total = i.Totals.Subtotal }, ErrInconsistentTotals},
		"línea alterada":         {func(i *Invoice, _ *IssueInput) { i.Lines[0].Tax = i.Lines[0].Subtotal }, ErrInconsistentTotals},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			inv, in := issuable(t), issueInput()
			c.mut(&inv, &in)
			if _, err := inv.Issue(in); !errors.Is(err, c.want) {
				t.Fatalf("err = %v, se esperaba %v", err, c.want)
			}
		})
	}
	for name, mut := range map[string]func(*IssueInput){
		"sin número":        func(in *IssueInput) { in.Number = " " },
		"número largo":      func(in *IssueInput) { in.Number = "FAC-" + string(make([]byte, 60)) },
		"sin razón social":  func(in *IssueInput) { in.Customer.LegalName = "" },
		"fecha mal formada": func(in *IssueInput) { in.IssueDate = "24/09/2026" },
	} {
		t.Run(name, func(t *testing.T) {
			in := issueInput()
			mut(&in)
			if _, err := issuable(t).Issue(in); err == nil {
				t.Fatal("debió fallar")
			}
		})
	}
}
