// Package certificate tiene las reglas del certificado de firma de la organización (la llave criptográfica de
// Hacienda o un certificado de la Ley 8454). Leer el .p12 es infraestructura (adapter pkcs12); aquí solo se decide
// si lo leído sirve para firmar comprobantes.
package certificate

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrUnreadable: el archivo no es un PKCS#12 o el PIN no lo abre (problem certificate-invalid). Nunca dice cuál de
	// las dos: no se ayuda a adivinar el PIN.
	ErrUnreadable = errors.New("no se pudo abrir el certificado con ese PIN")
	// ErrKeyNotAllowed: la llave no es RSA 2048 ni RSA 4096 (certificate-invalid).
	ErrKeyNotAllowed = errors.New("la llave del certificado debe ser RSA de 2048 o 4096 bits (Anexo 2)")
	// ErrNotYetValid y ErrExpired: fuera de vigencia (certificate-expired).
	ErrNotYetValid = errors.New("el certificado todavía no está vigente")
	ErrExpired     = errors.New("el certificado está vencido")
)

// Info es lo que se guarda en fiscal.certificates. Nunca la llave privada ni el PIN.
type Info struct {
	Subject      string
	Issuer       string
	SerialNumber string
	NotBefore    time.Time
	NotAfter     time.Time
	KeyAlgorithm string // "RSA" u otro
	KeyBits      int
}

// Validate decide si el certificado sirve para firmar hoy. Anexo 2 (pág. 84): «certificados con algoritmos de
// encriptación de tipo RSA 2048 o RSA 4096»; la llave criptográfica de Hacienda es RSA 2048 + SHA-256 (pág. 85).
// TODO(fiscal): verificar que el certificado sea del mismo contribuyente del perfil (el serialNumber del sujeto trae
// la cédula); el formato exacto no está en docs/hacienda/ y se confirma con la llave de pruebas.
func Validate(i Info, now time.Time) error {
	if i.KeyAlgorithm != "RSA" || (i.KeyBits != 2048 && i.KeyBits != 4096) {
		return fmt.Errorf("%w: %s de %d bits", ErrKeyNotAllowed, i.KeyAlgorithm, i.KeyBits)
	}
	if now.Before(i.NotBefore) {
		return ErrNotYetValid
	}
	if !now.Before(i.NotAfter) {
		return ErrExpired
	}
	return nil
}
