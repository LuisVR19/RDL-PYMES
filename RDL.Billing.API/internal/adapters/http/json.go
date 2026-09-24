package http

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"rdl/billing-api/internal/adapters/http/problem"
	"rdl/billing-api/internal/app"
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
	// Un cuerpo que es un arreglo (PUT /lines) lo valida el handler envuelto en un struct: validator solo recorre structs.
	if reflect.Indirect(reflect.ValueOf(dst)).Kind() != reflect.Struct {
		return nil
	}
	return validateStruct(dst)
}

// validateStruct aplica las etiquetas `validate` y nombra los campos como los ve el cliente (JSON).
func validateStruct(dst any) error {
	if err := validate.Struct(dst); err != nil {
		var verrs validator.ValidationErrors
		if errors.As(err, &verrs) {
			fields := make([]problem.FieldError, 0, len(verrs))
			for _, fe := range verrs {
				fields = append(fields, problem.FieldError{Field: jsonPath(fe), Message: validationMessage(fe)})
			}
			return validationError{fields: fields}
		}
		return err
	}
	return nil
}

// jsonPath convierte "createCustomerRequest.identification.number" en "identification.number".
func jsonPath(fe validator.FieldError) string {
	_, path, found := strings.Cut(fe.Namespace(), ".")
	if !found {
		return fe.Field()
	}
	return path
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
	case "email":
		return "debe ser un email válido"
	case "max":
		return "supera la longitud máxima de " + fe.Param()
	case "min":
		return "no alcanza la longitud mínima de " + fe.Param()
	default:
		return "no es válido"
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

const idempotencyHeader = "Idempotency-Key"

var errIdempotencyKeyMissing = errors.New("falta el header Idempotency-Key")

// idempotencyKey exige el header en los comandos POST: 1 a 255 caracteres ASCII visibles (convenciones §7).
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

// pathID lee {id}. Un id mal formado responde 404, igual que uno inexistente o de otra organización.
func pathID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil, app.ErrNotFound
	}
	return id, nil
}

// parsePage lee limit y cursor, comunes a todos los listados paginados (convenciones §9).
func parsePage(r *http.Request) (limit int, after *app.PageCursor, fields []problem.FieldError) {
	values := r.URL.Query()
	if v := values.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > app.MaxPageSize {
			fields = append(fields, problem.FieldError{Field: "limit", Message: "debe ser un entero entre 1 y " + strconv.Itoa(app.MaxPageSize)})
		}
		limit = n
	}
	if v := values.Get("cursor"); v != "" {
		c, err := decodeCursor(v)
		if err != nil {
			fields = append(fields, problem.FieldError{Field: "cursor", Message: "no es un cursor válido"})
		}
		after = &c
	}
	return limit, after, fields
}

func parseBoolParam(r *http.Request, name string, fields *[]problem.FieldError) *bool {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		*fields = append(*fields, problem.FieldError{Field: name, Message: "debe ser true o false"})
		return nil
	}
	return &b
}

// El cursor es opaco para el cliente: base64url de la posición del último elemento devuelto.
type cursorPayload struct {
	At time.Time `json:"a"`
	ID uuid.UUID `json:"i"`
}

func encodeCursor(c app.PageCursor) string {
	raw, _ := json.Marshal(cursorPayload{At: c.At, ID: c.ID})
	return base64.RawURLEncoding.EncodeToString(raw)
}

var errBadCursor = errors.New("cursor inválido")

func decodeCursor(s string) (app.PageCursor, error) {
	if len(s) > 512 {
		return app.PageCursor{}, errBadCursor
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return app.PageCursor{}, errBadCursor
	}
	var p cursorPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.ID == uuid.Nil || p.At.IsZero() {
		return app.PageCursor{}, errBadCursor
	}
	return app.PageCursor{At: p.At, ID: p.ID}, nil
}
