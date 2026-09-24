package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"rdl/platform-api/internal/adapters/http/problem"
	"rdl/platform-api/internal/app"
	"rdl/platform-api/internal/domain/membership"
	"rdl/platform-api/pkg/tenancy"
)

type (
	listMembers interface {
		Execute(ctx context.Context, t tenancy.Context, q app.MemberQuery) (app.MemberPage, error)
	}
	updateMember interface {
		Execute(ctx context.Context, t tenancy.Context, userID uuid.UUID, c membership.Change) (membership.Member, error)
	}
)

type MemberHandlers struct {
	List   listMembers
	Update updateMember
}

type memberResponse struct {
	UserID   uuid.UUID `json:"userId"`
	Email    string    `json:"email"`
	FullName string    `json:"fullName"`
	Status   string    `json:"status"`
	Roles    []string  `json:"roles"`
	JoinedAt time.Time `json:"joinedAt"`
}

type memberPageResponse struct {
	Items      []memberResponse `json:"items"`
	NextCursor *string          `json:"nextCursor"`
}

type updateMemberRequest struct {
	Role   *string `json:"role" validate:"omitnil,oneof=owner admin biller collector accountant read_only"`
	Status *string `json:"status" validate:"omitnil,oneof=active suspended"`
}

func (h *MemberHandlers) register(rt *routes, fail func(http.ResponseWriter, *http.Request, error)) {
	rt.handle("GET /v1/organizations/current/users", func(w http.ResponseWriter, r *http.Request) {
		q, err := parseMemberQuery(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		t, _ := tenancy.From(r.Context())
		page, err := h.List.Execute(r.Context(), t, q)
		if err != nil {
			fail(w, r, err)
			return
		}
		out := memberPageResponse{Items: make([]memberResponse, 0, len(page.Items))}
		for _, m := range page.Items {
			out.Items = append(out.Items, toMemberResponse(m))
		}
		if page.Next != nil {
			c := encodeCursor(*page.Next)
			out.NextCursor = &c
		}
		writeJSON(w, http.StatusOK, out)
	})

	rt.handle("PATCH /v1/organizations/current/users/{userId}", func(w http.ResponseWriter, r *http.Request) {
		userID, err := uuid.Parse(r.PathValue("userId"))
		if err != nil {
			// Un id mal formado se responde igual que uno inexistente: no hay nada que ver ahí.
			fail(w, r, app.ErrNotFound)
			return
		}
		var req updateMemberRequest
		if err := decodeJSON(w, r, &req); err != nil {
			fail(w, r, err)
			return
		}
		var c membership.Change
		if req.Role != nil {
			role := membership.Role(*req.Role)
			c.Role = &role
		}
		if req.Status != nil {
			status := membership.Status(*req.Status)
			c.Status = &status
		}
		t, _ := tenancy.From(r.Context())
		m, err := h.Update.Execute(r.Context(), t, userID, c)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, toMemberResponse(m))
	})
}

func toMemberResponse(m membership.Member) memberResponse {
	roles := make([]string, 0, len(m.Roles))
	for _, r := range m.Roles {
		roles = append(roles, string(r))
	}
	return memberResponse{
		UserID: m.UserID, Email: m.Email, FullName: m.FullName, Status: string(m.Status), Roles: roles, JoinedAt: m.JoinedAt.UTC(),
	}
}

func parseMemberQuery(r *http.Request) (app.MemberQuery, error) {
	limit, after, fields := parsePage(r)
	q := app.MemberQuery{Limit: limit, After: after}
	if v := r.URL.Query().Get("status"); v != "" {
		q.Status = membership.Status(v)
		if !q.Status.Valid() {
			fields = append(fields, problem.FieldError{Field: "status", Message: "debe ser active o suspended"})
		}
	}
	if len(fields) > 0 {
		return app.MemberQuery{}, validationError{fields: fields}
	}
	return q, nil
}

// parsePage lee limit y cursor, comunes a todos los listados paginados.
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

func encodeCursor(c app.PageCursor) string {
	raw, _ := json.Marshal(cursorPayload{At: c.At, ID: c.ID})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(s string) (app.PageCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return app.PageCursor{}, err
	}
	var p cursorPayload
	if err := json.Unmarshal(raw, &p); err != nil || p.ID == uuid.Nil || p.At.IsZero() {
		return app.PageCursor{}, errMalformed
	}
	return app.PageCursor{At: p.At, ID: p.ID}, nil
}
