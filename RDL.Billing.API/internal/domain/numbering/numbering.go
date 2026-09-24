// Package numbering modela las secuencias de número visible (billing.document_sequences): el número que ve el
// negocio (FAC-00000001), no el consecutivo fiscal de Hacienda, que es de E-Invoice.
package numbering

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// Digits: el número va con ceros a la izquierda hasta 8 dígitos (informe 0001 §4.5). Pasado 99 999 999 crece solo.
const Digits = 8

// MaxNextNumber: el número visible no puede pasar de 50 caracteres (InvoiceIssued.invoiceNumber); con 10 de prefijo,
// 15 dígitos es de sobra y cabe en bigint.
const MaxNextNumber = 999_999_999_999_999

var (
	// ErrInUse: la secuencia ya asignó números; cambiar prefijo o número inicial rompería la serie (409 sequence-in-use).
	ErrInUse = errors.New("numbering: la secuencia ya se usó")
	// ErrPrefixTaken: otra secuencia del mismo tipo usa el prefijo; sus números chocarían en invoices_number_uk (409).
	ErrPrefixTaken = errors.New("numbering: otra secuencia del mismo tipo usa ese prefijo")
	ErrInvalid     = errors.New("numbering: datos inválidos")
)

type FieldError struct{ Field, Message string }

func (e FieldError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Message) }
func (e FieldError) Unwrap() error { return ErrInvalid }

// prefixPattern: letras, dígitos, guion, guion bajo, punto o barra; hasta 10 (DocumentSequence.prefix del contrato).
// Vacío es válido (números sin prefijo).
var prefixPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]{0,10}$`)

// Scope identifica una secuencia: tipo de documento y sucursal (nil = la de toda la organización).
type Scope struct {
	DocumentType string
	BranchID     *uuid.UUID
}

func (s Scope) Equal(o Scope) bool {
	if s.DocumentType != o.DocumentType {
		return false
	}
	if s.BranchID == nil || o.BranchID == nil {
		return s.BranchID == o.BranchID
	}
	return *s.BranchID == *o.BranchID
}

type Sequence struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Scope
	Prefix     string
	NextNumber int64
	// LastAssigned es nil mientras la secuencia no asignó ningún número.
	LastAssigned *int64
	UpdatedAt    time.Time
}

// New es una secuencia sin configurar: sin prefijo, empieza en 1.
func New(organizationID uuid.UUID, scope Scope) Sequence {
	return Sequence{OrganizationID: organizationID, Scope: scope, NextNumber: 1}
}

func (s Sequence) Used() bool { return s.LastAssigned != nil }

// Configure fija prefijo y próximo número. Solo si la secuencia nunca se usó, y sin repetir el prefijo de otra
// secuencia del mismo tipo (siblings: las demás secuencias de la organización).
func (s Sequence) Configure(prefix string, nextNumber int64, siblings []Sequence) (Sequence, error) {
	if s.Used() {
		return Sequence{}, ErrInUse
	}
	var errs []error
	if !prefixPattern.MatchString(prefix) {
		errs = append(errs, FieldError{Field: "prefix", Message: "hasta 10 letras, dígitos, punto, guion, guion bajo o barra"})
	}
	if nextNumber < 1 || nextNumber > MaxNextNumber {
		errs = append(errs, FieldError{Field: "nextNumber", Message: fmt.Sprintf("debe estar entre 1 y %d", int64(MaxNextNumber))})
	}
	if err := errors.Join(errs...); err != nil {
		return Sequence{}, err
	}
	for _, o := range siblings {
		if o.DocumentType == s.DocumentType && !o.Equal(s.Scope) && o.Prefix == prefix {
			return Sequence{}, ErrPrefixTaken
		}
	}
	next := s
	next.Prefix, next.NextNumber = prefix, nextNumber
	return next, nil
}

// Assign entrega el próximo número visible y la secuencia avanzada. Se llama bajo SELECT ... FOR UPDATE en la
// transacción de la emisión: si la transacción se revierte, el avance también (sin huecos por rollback, ADR 0006).
func (s Sequence) Assign() (string, Sequence) {
	n := s.NextNumber
	next := s
	next.LastAssigned = &n
	next.NextNumber = n + 1
	return Format(s.Prefix, n), next
}

// Format arma el número visible: prefijo + número con ceros a la izquierda hasta Digits.
func Format(prefix string, n int64) string {
	return fmt.Sprintf("%s%0*d", prefix, Digits, n)
}

// Choose elige la secuencia para emitir un documento: la de la sucursal del documento si existe, si no la de la
// organización (sin sucursal), y si tampoco existe, una nueva de la organización (informe 0001 §4.5).
func Choose(organizationID uuid.UUID, documentType string, branchID *uuid.UUID, existing []Sequence) Sequence {
	if branchID != nil {
		for _, s := range existing {
			if s.Equal(Scope{DocumentType: documentType, BranchID: branchID}) {
				return s
			}
		}
	}
	org := Scope{DocumentType: documentType}
	for _, s := range existing {
		if s.Equal(org) {
			return s
		}
	}
	return New(organizationID, org)
}
