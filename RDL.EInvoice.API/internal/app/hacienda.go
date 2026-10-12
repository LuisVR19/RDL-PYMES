package app

import (
	"context"
	"errors"
	"time"
)

// HaciendaClient es el API de recepción de comprobantes de Hacienda (Anexo 3, págs. 97–99): POST /recepcion y
// GET /recepcion/{clave}. El adapter real (OAuth2 con token de 5 minutos, solo sandbox) llega con el incremento 9;
// hasta entonces el stub de internal/adapters/hacienda cierra el flujo vertical (planning P5, F3).
type HaciendaClient interface {
	// Submit envía el comprobante firmado. ErrHaciendaUnavailable si no respondió (timeout o 5xx): se reintenta.
	// ErrHaciendaRejectedRequest si respondió 4xx: la petición está mal y reintentarla igual no sirve.
	Submit(ctx context.Context, s Submission) error
	// Status consulta el estado de un comprobante ya enviado.
	Status(ctx context.Context, key string) (HaciendaStatus, error)
}

// Submission es lo que se envía a POST /recepcion. Los nombres de los campos del JSON los fija la documentación
// web del API, que no está en docs/hacienda/ (informe 0001 §4): TODO(fiscal).
type Submission struct {
	Key                    string // clave de 50 caracteres
	IssuedAt               time.Time
	IssuerIdentification   string
	IssuerIdentificationTp string
	SignedXML              []byte
}

// HaciendaOutcome es el resultado de la validación, independiente de cómo lo exprese el API.
type HaciendaOutcome string

const (
	// HaciendaPending: recibido y en proceso (el ejemplo del Anexo 3, pág. 99, trae indEstado "RECIBIDO").
	HaciendaPending HaciendaOutcome = "pending"
	// HaciendaAccepted: MensajeHacienda con Mensaje 1 «Aceptado» (Nota 11, pág. 76).
	HaciendaAccepted HaciendaOutcome = "accepted"
	// HaciendaRejected: MensajeHacienda con Mensaje 3 «Rechazado» (Nota 11, pág. 76).
	HaciendaRejected HaciendaOutcome = "rejected"
)

// HaciendaStatus es la respuesta de GET /recepcion/{clave}.
type HaciendaStatus struct {
	Outcome     HaciendaOutcome
	Message     string // detalle de Hacienda, tal como llegó (va a hacienda_status_message y al evento Rejected)
	ResponseXML []byte // MensajeHacienda firmado; se guarda en el BlobStore (incremento 9)
	ReceivedAt  time.Time
}

var (
	ErrHaciendaUnavailable     = errors.New("sin respuesta de Hacienda")
	ErrHaciendaRejectedRequest = errors.New("petición rechazada por Hacienda")
	ErrHaciendaUnknownKey      = errors.New("clave desconocida para Hacienda")
)
