// Package hacienda implementa app.HaciendaClient. Hoy solo el stub de desarrollo (planning P5, F3); el cliente real
// contra el ambiente de pruebas llega con el incremento 9.
package hacienda

import (
	"context"
	"fmt"
	"sync"
	"time"

	"rdl/einvoice-api/internal/app"
)

// Stub simula Hacienda en memoria y responde según el último carácter de la clave (que es el último del código de
// seguridad, Nota 3 inciso h): así una prueba elige el resultado armando la clave.
//
//	'8' → Submit no responde (ErrHaciendaUnavailable: timeout, se reintenta)
//	'9' → recibido y después rechazado
//	otro → recibido y después aceptado
//
// La primera consulta después del envío responde «pendiente», como un comprobante que Hacienda todavía valida
// (art. 11: hasta 3 horas, FUENTE: borrador sept-2024); las siguientes, el resultado final. Nunca habla con Hacienda.
type Stub struct {
	mu      sync.Mutex
	sent    map[string]int // clave → consultas hechas
	now     func() time.Time
	Rejects string // motivo de los rechazos simulados
}

func NewStub() *Stub {
	return &Stub{sent: map[string]int{}, now: time.Now, Rejects: "Rechazo simulado por el stub de Hacienda (no es una respuesta real)"}
}

func (s *Stub) Submit(ctx context.Context, sub app.Submission) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w: %w", app.ErrHaciendaUnavailable, err)
	}
	if len(sub.Key) != 50 {
		return fmt.Errorf("%w: la clave debe tener 50 caracteres", app.ErrHaciendaRejectedRequest)
	}
	if len(sub.SignedXML) == 0 {
		return fmt.Errorf("%w: falta el comprobante firmado", app.ErrHaciendaRejectedRequest)
	}
	if last(sub.Key) == '8' {
		return fmt.Errorf("%w: timeout simulado", app.ErrHaciendaUnavailable)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sent[sub.Key]; !ok {
		s.sent[sub.Key] = 0
	}
	return nil
}

func (s *Stub) Status(ctx context.Context, key string) (app.HaciendaStatus, error) {
	if err := ctx.Err(); err != nil {
		return app.HaciendaStatus{}, fmt.Errorf("%w: %w", app.ErrHaciendaUnavailable, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.sent[key]
	if !ok {
		return app.HaciendaStatus{}, app.ErrHaciendaUnknownKey
	}
	s.sent[key] = n + 1
	st := app.HaciendaStatus{ReceivedAt: s.now().UTC()}
	switch {
	case n == 0:
		st.Outcome = app.HaciendaPending
	case last(key) == '9':
		st.Outcome, st.Message = app.HaciendaRejected, s.Rejects
	default:
		st.Outcome = app.HaciendaAccepted
	}
	return st, nil
}

func last(key string) byte {
	if key == "" {
		return 0
	}
	return key[len(key)-1]
}

var _ app.HaciendaClient = (*Stub)(nil)
