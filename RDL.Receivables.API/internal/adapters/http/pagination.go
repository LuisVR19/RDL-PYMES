package http

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"rdl/receivables-api/internal/adapters/http/problem"
	"rdl/receivables-api/internal/app"
)

// Origen: RDL.Platform.API (internal/adapters/http/members.go), copiado sin cambios de comportamiento.

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

// El cursor es opaco para el cliente: base64url de la posición del último elemento devuelto.
type cursorPayload struct {
	At time.Time `json:"a"`
	ID uuid.UUID `json:"i"`
}

var errBadCursor = errors.New("cursor inválido")

func encodeCursor(c app.PageCursor) string {
	raw, _ := json.Marshal(cursorPayload{At: c.At, ID: c.ID})
	return base64.RawURLEncoding.EncodeToString(raw)
}

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
