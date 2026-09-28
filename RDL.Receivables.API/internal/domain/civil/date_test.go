package civil

import (
	"testing"
	"time"
)

func TestTodayUsesOrganizationZone(t *testing.T) {
	cr, err := time.LoadLocation("America/Costa_Rica") // UTC-6, sin horario de verano
	if err != nil {
		t.Fatal(err)
	}
	// 03:00 UTC del 25 todavía es el 24 en Costa Rica.
	now := time.Date(2026, 9, 25, 3, 0, 0, 0, time.UTC)
	if got := Today(now, cr); got.String() != "2026-09-24" {
		t.Fatalf("Today=%s, se esperaba 2026-09-24", got)
	}
	if got := Today(now, time.UTC); got.String() != "2026-09-25" {
		t.Fatalf("Today(UTC)=%s", got)
	}
}

func TestParseAndCompare(t *testing.T) {
	a, err := Parse("2026-02-28")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Parse("2026-03-01")
	if !a.Before(b) || b.Before(a) || a.Before(a) {
		t.Fatal("Before no ordena por día")
	}
	for _, bad := range []string{"", "2026-02-30", "26-02-28", "2026-02-28T00:00:00Z"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) debió fallar", bad)
		}
	}
}

func TestFromTimeKeepsTheDayOfAPostgresDate(t *testing.T) {
	d := FromTime(time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC))
	if d.String() != "2026-09-24" || d.Time() != time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("d=%s", d)
	}
	if d.IsZero() || !(Date{}).IsZero() {
		t.Fatal("IsZero")
	}
}
