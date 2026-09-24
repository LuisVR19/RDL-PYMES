package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"rdl/platform-api/internal/adapters/http/problem"
)

const maxBodyBytes = 1 << 20

var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New(validator.WithRequiredStructEnabled())
	// Los errores de validación nombran el campo como lo ve el cliente (JSON), no como se llama en Go.
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" {
			return ""
		}
		return name
	})
	return v
}

// errMalformed: el cuerpo no es JSON válido para el endpoint (400). Los errores de validación de campos son 422.
var errMalformed = errors.New("cuerpo de la petición inválido")

type validationError struct{ fields []problem.FieldError }

func (e validationError) Error() string { return "validación fallida" }

// decodeJSON lee exactamente un objeto JSON, rechaza campos desconocidos y valida las etiquetas `validate`.
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
	if err := validate.Struct(dst); err != nil {
		var verrs validator.ValidationErrors
		if errors.As(err, &verrs) {
			fields := make([]problem.FieldError, 0, len(verrs))
			for _, fe := range verrs {
				fields = append(fields, problem.FieldError{Field: fe.Field(), Message: validationMessage(fe)})
			}
			return validationError{fields: fields}
		}
		return err
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

func validationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "es obligatorio"
	case "uuid", "uuid4":
		return "debe ser un UUID"
	case "email":
		return "debe ser un email válido"
	case "max":
		return "supera la longitud máxima de " + fe.Param()
	case "min":
		return "no alcanza la longitud mínima de " + fe.Param()
	case "oneof":
		return "debe ser uno de: " + fe.Param()
	default:
		return "no es válido"
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
