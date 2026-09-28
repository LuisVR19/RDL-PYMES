// Package civil modela fechas de negocio (issued_on, due_on, received_on, asOf): un día del calendario, sin hora
// ni zona. "Hoy" siempre se calcula en la zona de la organización, nunca en UTC.
package civil

import (
	"fmt"
	"time"
)

const layout = "2006-01-02"

// Date es un día del calendario. El valor cero no es una fecha válida (IsZero).
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// Today es la fecha de negocio de now en la zona loc (core.organizations.timezone).
func Today(now time.Time, loc *time.Location) Date {
	return FromTime(now.In(loc))
}

// FromTime toma el día de t en su propia zona. pgx escanea una columna date como medianoche UTC de ese día.
func FromTime(t time.Time) Date {
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

// Parse lee AAAA-MM-DD (el formato BusinessDate del contrato).
func Parse(s string) (Date, error) {
	t, err := time.Parse(layout, s)
	if err != nil {
		return Date{}, fmt.Errorf("civil: fecha inválida %q", s)
	}
	return FromTime(t), nil
}

func (d Date) IsZero() bool { return d == Date{} }

// Time es la medianoche UTC del día: la forma en que se envía a una columna date.
func (d Date) Time() time.Time { return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC) }

func (d Date) String() string { return d.Time().Format(layout) }

func (d Date) Before(o Date) bool { return d.Time().Before(o.Time()) }
