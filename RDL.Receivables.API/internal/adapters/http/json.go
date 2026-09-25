package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"bitbucket.org/rdl/contracts/pkg/events/money"

	"rdl/receivables-api/internal/adapters/http/problem"
	"rdl/receivables-api/internal/domain/civil"
)

const maxBodyBytes = 1 << 20

// errMalformed: el cuerpo no es JSON válido para el endpoint (400). Los errores de campo son 422.
var errMalformed = errors.New("cuerpo de la petición inválido")

// decodeJSON lee exactamente un objeto JSON y rechaza campos desconocidos (Origen: RDL.Platform.API). La validación
// de cada campo la hace el handler con fields, para responder todos los errores juntos.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%w: %s", errMalformed, describeJSONError(err))
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: se esperaba un único objeto JSON", errMalformed)
	}
	return nil
}

// describeJSONError da un mensaje útil sin repetir el contenido enviado.
func describeJSONError(err error) string {
	var syntax *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	var maxErr *http.MaxBytesError
	switch {
	case errors.As(err, &syntax):
		return fmt.Sprintf("JSON mal formado en la posición %d", syntax.Offset)
	case errors.As(err, &typeErr):
		return fmt.Sprintf("el campo %q tiene un tipo inválido", typeErr.Field)
	case errors.As(err, &maxErr):
		return "el cuerpo supera el tamaño máximo"
	case errors.Is(err, io.EOF):
		return "el cuerpo está vacío"
	case strings.HasPrefix(err.Error(), "json: unknown field "):
		return "campo no permitido " + strings.TrimPrefix(err.Error(), "json: unknown field ")
	default:
		return "JSON inválido"
	}
}

// fields junta los errores de validación de una petición y los devuelve como un solo 422.
type fields struct{ errs []problem.FieldError }

func (f *fields) add(field, msg string) {
	f.errs = append(f.errs, problem.FieldError{Field: field, Message: msg})
}

func (f *fields) err() error {
	if len(f.errs) == 0 {
		return nil
	}
	return validationError{fields: f.errs}
}

// uuidField: obligatorio y distinto de cero.
func (f *fields) uuid(field, v string) uuid.UUID {
	id, err := uuid.Parse(v)
	if err != nil || id == uuid.Nil {
		f.add(field, "debe ser un UUID")
		return uuid.Nil
	}
	return id
}

// money lee un monto del contrato (Money: string decimal, hasta 13 enteros y 5 decimales) sin pasar por float.
// Positivo: el dominio exige > 0 en todo monto de una operación.
func (f *fields) money(field, v string) decimal.Decimal {
	if _, err := money.ParseAmount(v); err != nil {
		f.add(field, "debe ser un monto decimal en texto, con hasta 5 decimales (ej. \"1300.50\")")
		return decimal.Zero
	}
	d := decimal.RequireFromString(v)
	if !d.IsPositive() {
		f.add(field, "debe ser mayor que cero")
	}
	return d
}

func (f *fields) date(field, v string) civil.Date {
	d, err := civil.Parse(v)
	if err != nil {
		f.add(field, "debe ser una fecha AAAA-MM-DD")
	}
	return d
}

func (f *fields) text(field, v string, minLen, maxLen int) string {
	v = strings.TrimSpace(v)
	switch {
	case len(v) < minLen && minLen == 1:
		f.add(field, "es obligatorio")
	case len([]rune(v)) > maxLen:
		f.add(field, fmt.Sprintf("supera la longitud máxima de %d", maxLen))
	}
	return v
}

// pathID lee el {id} de la ruta. Un id mal formado es 404: para el cliente es un recurso que no existe.
func pathID(r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	return id, err == nil && id != uuid.Nil
}

const idempotencyHeader = "Idempotency-Key"

var errIdempotencyKeyMissing = errors.New("falta el header Idempotency-Key")

// idempotencyKey exige el header en los comandos POST: 1 a 255 caracteres ASCII visibles (Origen: Platform).
func idempotencyKey(r *http.Request) (string, error) {
	k := strings.TrimSpace(r.Header.Get(idempotencyHeader))
	if k == "" || len(k) > 255 {
		return "", errIdempotencyKeyMissing
	}
	for _, c := range k {
		if c < 0x21 || c > 0x7e {
			return "", errIdempotencyKeyMissing
		}
	}
	return k, nil
}
