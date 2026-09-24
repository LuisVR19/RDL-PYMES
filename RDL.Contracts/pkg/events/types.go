package events

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Instant es un instante que siempre se serializa en UTC con sufijo Z (schemas/common/utc-datetime.json).
// Al leer, rechaza un desfase distinto de Z en lugar de convertirlo en silencio.
type Instant struct{ t time.Time }

func NewInstant(t time.Time) Instant { return Instant{t: t.UTC()} }

func (i Instant) Time() time.Time { return i.t }
func (i Instant) IsZero() bool    { return i.t.IsZero() }

func (i Instant) MarshalJSON() ([]byte, error) {
	if i.t.IsZero() {
		return nil, fmt.Errorf("%w: instante sin valor", ErrInvalid)
	}
	return json.Marshal(i.t.UTC().Format(time.RFC3339Nano))
}

func (i *Instant) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("%w: instante: se esperaba un string", ErrInvalid)
	}
	if !strings.HasSuffix(s, "Z") {
		return fmt.Errorf("%w: instante %q sin sufijo Z", ErrInvalid, s)
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return fmt.Errorf("%w: instante %q: %w", ErrInvalid, s, err)
	}
	i.t = t.UTC()
	return nil
}

// Date es una fecha de negocio sin hora (YYYY-MM-DD), interpretada en la zona horaria de la organización.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// DateIn devuelve la fecha de negocio de un instante en la zona dada (la de la organización).
func DateIn(t time.Time, loc *time.Location) Date {
	y, m, d := t.In(loc).Date()
	return Date{Year: y, Month: m, Day: d}
}

func ParseDate(s string) (Date, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return Date{}, fmt.Errorf("%w: fecha %q (YYYY-MM-DD)", ErrInvalid, s)
	}
	return Date{Year: t.Year(), Month: t.Month(), Day: t.Day()}, nil
}

func (d Date) String() string { return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day) }
func (d Date) IsZero() bool   { return d == Date{} }

// Before compara fechas de negocio (por ejemplo, dueDate no puede ser anterior a issueDate).
func (d Date) Before(o Date) bool {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC).Before(time.Date(o.Year, o.Month, o.Day, 0, 0, 0, 0, time.UTC))
}

func (d Date) MarshalJSON() ([]byte, error) {
	if d.IsZero() {
		return nil, fmt.Errorf("%w: fecha sin valor", ErrInvalid)
	}
	return json.Marshal(d.String())
}

func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("%w: fecha: se esperaba un string", ErrInvalid)
	}
	v, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = v
	return nil
}

// Service es el nombre de máquina de un servicio (schemas/common/service-name.json).
type Service string

const (
	ServicePlatform    Service = "platform"
	ServiceBilling     Service = "billing"
	ServiceFiscal      Service = "fiscal"
	ServiceReceivables Service = "receivables"
)
