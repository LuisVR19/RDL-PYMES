// Package taxpayer es el perfil del contribuyente emisor: los datos del nodo Emisor del comprobante (Anexo 1,
// págs. 19–22) y el ambiente de Hacienda. Reglas puras, sin SQL ni HTTP.
package taxpayer

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Environment es el ambiente de Hacienda de la organización (CHECK taxpayer_profiles_environment_ck).
type Environment string

const (
	EnvironmentSandbox    Environment = "sandbox"
	EnvironmentProduction Environment = "production"
)

// Códigos de tipo de identificación del emisor que admite una factura, nota de crédito o de débito. Anexo 1,
// Nota 4 (pág. 69): 05 y 06 solo existen en la factura electrónica de compra (pág. 19), fuera de V1 (decisión 1).
var issuerIdentificationTypes = map[string]bool{"01": true, "02": true, "03": true, "04": true}

// Formato del número por tipo, Anexo 1 pág. 20: física 9 dígitos sin cero al inicio; jurídica 10 caracteres
// (alfanumérica desde la bitácora del 22/04/2026, pág. 3); DIMEX 11 o 12 dígitos sin cero al inicio; NITE 10 dígitos.
// Sin guiones en ningún caso.
var identificationFormats = map[string]*regexp.Regexp{
	"01": regexp.MustCompile(`^[1-9][0-9]{8}$`),
	"02": regexp.MustCompile(`^[0-9A-Za-z]{10}$`),
	"03": regexp.MustCompile(`^[1-9][0-9]{10,11}$`),
	"04": regexp.MustCompile(`^[0-9]{10}$`),
}

// emailRe es la expresión regular del correo electrónico del Anexo 1, pág. 22, tal como la publica (sin los escapes
// del PDF).
var emailRe = regexp.MustCompile(`^\s*(([^<>()\[\]\.,;:\s@"]+(\.[^<>()\[\]\.,;:\s@"]+)*)|(".+"))@(([^<>()\[\]\.,;:\s@"]+\.)+[^<>()\[\]\.,;:\s@"]{0,})\s*$`)

var (
	digitsRe   = regexp.MustCompile(`^[0-9]+$`)
	activityRe = regexp.MustCompile(`^[0-9A-Za-z]{6}$`)
	provinceRe = regexp.MustCompile(`^[0-9]$`)
	twoDigitRe = regexp.MustCompile(`^[0-9]{2}$`)
)

// Location es la ubicación del emisor (Emisor>Ubicacion, Anexo 1 pág. 21; códigos de la Nota 14).
type Location struct {
	ProvinceCode   string
	CantonCode     string
	DistrictCode   string
	AddressDetails string
}

// Profile es lo que la organización configura. Los secretos (credenciales de Hacienda, certificado y PIN) no viven
// aquí: van al SecretStore y el perfil solo guarda su referencia.
type Profile struct {
	Environment            Environment
	IdentificationTypeCode string
	IdentificationNumber   string
	LegalName              string
	TradeName              string
	Email                  string
	Phone                  string
	Location               *Location
	EconomicActivityCode   string
}

// FieldError es un campo inválido; el adapter HTTP los devuelve juntos en un 422.
type FieldError struct {
	Field   string
	Message string
}

// Validate aplica las reglas de formato del nodo Emisor. Las de existencia en catálogo (tipo de identificación,
// ubicación, actividad económica) las verifica el caso de uso contra la base.
func (p Profile) Validate() []FieldError {
	var errs []FieldError
	add := func(field, msg string) { errs = append(errs, FieldError{Field: field, Message: msg}) }

	switch p.Environment {
	case EnvironmentSandbox:
	case EnvironmentProduction:
		// Prompt P5: prohibido el ambiente de producción de Hacienda en V1. Imposible de confundir: ni se guarda.
		add("environment", "solo se admite sandbox: el ambiente de producción de Hacienda no está habilitado")
	default:
		add("environment", "debe ser sandbox o production")
	}

	if !issuerIdentificationTypes[p.IdentificationTypeCode] {
		add("identification.typeCode", "debe ser 01 (física), 02 (jurídica), 03 (DIMEX) o 04 (NITE) (Nota 4)")
	} else if !identificationFormats[p.IdentificationTypeCode].MatchString(p.IdentificationNumber) {
		add("identification.number", identificationHint(p.IdentificationTypeCode))
	}

	// Nombre o razón social: mínimo 5, máximo 100 (pág. 19). Nombre comercial: mínimo 3, máximo 80 (pág. 21).
	if n := utf8.RuneCountInString(strings.TrimSpace(p.LegalName)); n < 5 || n > 100 {
		add("legalName", "debe tener de 5 a 100 caracteres")
	}
	if p.TradeName != "" {
		if n := utf8.RuneCountInString(strings.TrimSpace(p.TradeName)); n < 3 || n > 80 {
			add("tradeName", "debe tener de 3 a 80 caracteres")
		}
	}
	// Correo: máximo 160 y la expresión regular de la pág. 22.
	if utf8.RuneCountInString(p.Email) > 160 || !emailRe.MatchString(p.Email) {
		add("email", "no es un correo electrónico válido (Anexo 1, pág. 22)")
	}
	// Teléfono: NumTelefono es Integer de 8 a 20 dígitos (pág. 22). Se guarda sin código de país.
	if p.Phone != "" && (!digitsRe.MatchString(p.Phone) || len(p.Phone) < 8 || len(p.Phone) > 20) {
		add("phone", "debe tener de 8 a 20 dígitos, sin código de país ni separadores")
	}
	if l := p.Location; l != nil {
		if !provinceRe.MatchString(l.ProvinceCode) {
			add("location.provinceCode", "debe ser un código de 1 dígito (Nota 14)")
		}
		if !twoDigitRe.MatchString(l.CantonCode) {
			add("location.cantonCode", "debe ser un código de 2 dígitos (Nota 14)")
		}
		if !twoDigitRe.MatchString(l.DistrictCode) {
			add("location.districtCode", "debe ser un código de 2 dígitos (Nota 14)")
		}
		// Otras señas: 5 a 250, obligatorias si hay ubicación (pág. 21). El barrio es opcional en la factura (pág. 21)
		// y taxpayer_profiles no tiene dónde guardarlo: no se pide.
		if n := utf8.RuneCountInString(strings.TrimSpace(l.AddressDetails)); n < 5 || n > 250 {
			add("location.addressDetails", "otras señas: de 5 a 250 caracteres")
		}
	}
	// Actividad económica: 6 caracteres (pág. 18); su existencia en el padrón la verifica el caso de uso.
	if p.EconomicActivityCode != "" && !activityRe.MatchString(p.EconomicActivityCode) {
		add("economicActivityCode", "debe tener 6 caracteres (padrón del RUT)")
	}
	return errs
}

func identificationHint(typeCode string) string {
	switch typeCode {
	case "01":
		return "cédula física: 9 dígitos, sin cero al inicio ni guiones (Anexo 1, pág. 20)"
	case "02":
		return "cédula jurídica: 10 caracteres, sin guiones (Anexo 1, pág. 20)"
	case "03":
		return "DIMEX: 11 o 12 dígitos, sin cero al inicio ni guiones (Anexo 1, pág. 20)"
	default:
		return fmt.Sprintf("NITE: 10 dígitos, sin guiones (Anexo 1, pág. 20); tipo %s", typeCode)
	}
}
