package hacienda

import (
	"context"
	"errors"
	"strings"
	"testing"

	"rdl/einvoice-api/internal/app"
)

func key(lastChar string) string { return strings.Repeat("5", 49) + lastChar }

func submit(t *testing.T, s *Stub, k string) error {
	t.Helper()
	return s.Submit(context.Background(), app.Submission{Key: k, SignedXML: []byte("<FacturaElectronica/>")})
}

func TestStubOutcomesByKey(t *testing.T) {
	s := NewStub()
	for _, c := range []struct {
		last string
		want app.HaciendaOutcome
	}{{"1", app.HaciendaAccepted}, {"9", app.HaciendaRejected}} {
		k := key(c.last)
		if err := submit(t, s, k); err != nil {
			t.Fatalf("%s: submit: %v", c.last, err)
		}
		first, err := s.Status(context.Background(), k)
		if err != nil || first.Outcome != app.HaciendaPending {
			t.Fatalf("%s: primera consulta = %+v, %v; se esperaba pendiente", c.last, first, err)
		}
		final, err := s.Status(context.Background(), k)
		if err != nil || final.Outcome != c.want {
			t.Fatalf("%s: consulta final = %+v, %v; se esperaba %s", c.last, final, err, c.want)
		}
		if c.want == app.HaciendaRejected && final.Message == "" {
			t.Error("un rechazo trae su motivo")
		}
	}
}

func TestStubTimeoutAndBadRequests(t *testing.T) {
	s := NewStub()
	if err := submit(t, s, key("8")); !errors.Is(err, app.ErrHaciendaUnavailable) {
		t.Errorf("clave terminada en 8: err = %v, se esperaba ErrHaciendaUnavailable", err)
	}
	if _, err := s.Status(context.Background(), key("8")); !errors.Is(err, app.ErrHaciendaUnknownKey) {
		t.Errorf("lo que no se recibió no existe: err = %v", err)
	}
	if err := submit(t, s, "corta"); !errors.Is(err, app.ErrHaciendaRejectedRequest) {
		t.Errorf("clave corta: err = %v", err)
	}
	if err := s.Submit(context.Background(), app.Submission{Key: key("1")}); !errors.Is(err, app.ErrHaciendaRejectedRequest) {
		t.Errorf("sin XML: err = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.Submit(ctx, app.Submission{Key: key("1"), SignedXML: []byte("x")}); !errors.Is(err, app.ErrHaciendaUnavailable) {
		t.Errorf("contexto cancelado: err = %v", err)
	}
}
