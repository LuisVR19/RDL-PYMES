// Package document es el agregado DocumentoElectrónico: estados, tipos de comprobante y el snapshot con el que se
// genera el XML. Sin SQL, sin HTTP y sin librerías de infraestructura.
package document

import (
	"errors"
	"fmt"
)

// Status es fiscal.electronic_documents.status (CHECK electronic_documents_status_ck).
type Status string

const (
	StatusProcessing  Status = "processing"
	StatusSigned      Status = "signed"
	StatusSent        Status = "sent"
	StatusAccepted    Status = "accepted"
	StatusRejected    Status = "rejected"
	StatusContingency Status = "contingency"
	StatusError       Status = "error"
)

// ErrInvalidTransition: el estado actual no admite el paso pedido. En HTTP es 409.
var ErrInvalidTransition = errors.New("el estado del documento no admite esa transición")

// transitions es RDL.Contracts/state-machines/electronic-document.yaml v1. Esas transiciones son una PROPUESTA del
// repo de contratos (C3); se confirman contra Hacienda en el incremento 9 y lo que cambie va como PR a contratos.
// TODO(fiscal): un documento en error o contingencia por mucho tiempo, ¿emite algún evento?
var transitions = map[Status][]Status{
	StatusProcessing:  {StatusSigned, StatusError},
	StatusSigned:      {StatusSent, StatusContingency},
	StatusContingency: {StatusSent},
	StatusSent:        {StatusAccepted, StatusRejected},
	StatusError:       {StatusProcessing},
}

// IsFinal: accepted y rejected no cambian más (también lo impone el trigger electronic_documents_guard).
func (s Status) IsFinal() bool { return s == StatusAccepted || s == StatusRejected }

// Valid indica si s es uno de los 7 estados de la base.
func (s Status) Valid() bool {
	switch s {
	case StatusProcessing, StatusSigned, StatusSent, StatusAccepted, StatusRejected, StatusContingency, StatusError:
		return true
	}
	return false
}

// CanTransition indica si la máquina de estados admite from → to.
func CanTransition(from, to Status) bool {
	for _, s := range transitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// Transition valida from → to y devuelve un error tipado si no se admite.
func Transition(from, to Status) error {
	if !CanTransition(from, to) {
		return fmt.Errorf("%w: %s → %s", ErrInvalidTransition, from, to)
	}
	return nil
}
