package aging

import (
	"testing"
	"time"

	"rdl/receivables-api/internal/domain/civil"
)

func date(s string) civil.Date {
	d, err := civil.Parse(s)
	if err != nil {
		panic(err)
	}
	return d
}

// Bordes de cada tramo. Vencer hoy no es atraso; el primer día de atraso es mañana.
func TestBuckets(t *testing.T) {
	due := date("2026-06-30")
	cases := []struct {
		asOf string
		days int
		want Bucket
	}{
		{"2026-06-01", 0, Current},
		{"2026-06-30", 0, Current},
		{"2026-07-01", 1, Days1_30},
		{"2026-07-30", 30, Days1_30},
		{"2026-07-31", 31, Days31_60},
		{"2026-08-29", 60, Days31_60},
		{"2026-08-30", 61, Days61_90},
		{"2026-09-28", 90, Days61_90},
		{"2026-09-29", 91, Days90Plus},
		{"2027-06-30", 365, Days90Plus},
	}
	for _, c := range cases {
		asOf := date(c.asOf)
		if got := DaysOverdue(due, asOf); got != c.days {
			t.Errorf("%s: días %d, se esperaba %d", c.asOf, got, c.days)
		}
		if got := For(due, asOf); got != c.want {
			t.Errorf("%s: tramo %s, se esperaba %s", c.asOf, got, c.want)
		}
	}
}

// La fecha de negocio sale de la zona de la organización: a las 20:00 de Costa Rica ya es el día siguiente en UTC,
// pero la cuenta que vence "hoy" sigue sin atraso.
func TestAsOfUsesOrganizationTimezone(t *testing.T) {
	cr, err := time.LoadLocation("America/Costa_Rica")
	if err != nil {
		t.Skip("sin zoneinfo:", err)
	}
	now := time.Date(2026, time.July, 1, 2, 0, 0, 0, time.UTC) // 30 de junio, 20:00 en Costa Rica
	if got := For(date("2026-06-30"), civil.Today(now, cr)); got != Current {
		t.Errorf("vence hoy en Costa Rica: %s", got)
	}
	if got := For(date("2026-06-30"), civil.Today(now, time.UTC)); got != Days1_30 {
		t.Errorf("control en UTC: %s", got)
	}
}

// Cruza un cambio de año y un año bisiesto sin perder ni sumar días.
func TestDaysAcrossLeapYear(t *testing.T) {
	if got := DaysOverdue(date("2028-02-28"), date("2028-03-01")); got != 2 {
		t.Errorf("bisiesto: %d", got)
	}
	if got := DaysOverdue(date("2026-12-31"), date("2027-01-01")); got != 1 {
		t.Errorf("año nuevo: %d", got)
	}
}
