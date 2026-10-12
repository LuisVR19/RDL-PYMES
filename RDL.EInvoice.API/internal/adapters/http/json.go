package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"rdl/einvoice-api/internal/adapters/http/problem"
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

// validationError agrupa los campos inválidos de una petición (422).
type validationError struct{ fields []problem.FieldError }

func (e validationError) Error() string { return "validación fallida" }

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

// uuid: obligatorio y distinto de cero.
func (f *fields) uuid(field, v string) uuid.UUID {
	id, err := uuid.Parse(v)
	if err != nil || id == uuid.Nil {
		f.add(field, "debe ser un UUID")
		return uuid.Nil
	}
	return id
}

// pathID lee el {id} de la ruta. Un id mal formado es 404: para el cliente es un recurso que no existe.
func pathID(r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	return id, err == nil && id != uuid.Nil
}
