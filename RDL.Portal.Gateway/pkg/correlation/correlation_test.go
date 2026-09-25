package correlation

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func run(t *testing.T, incoming string) (header string, inCtx uuid.UUID) {
	t.Helper()
	h := Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		id, ok := FromContext(r.Context())
		if !ok {
			t.Fatal("sin correlation id en el contexto")
		}
		inCtx = id
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if incoming != "" {
		req.Header.Set(Header, incoming)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Header().Get(Header), inCtx
}

func TestPropagatesValidIncomingID(t *testing.T) {
	want := uuid.New()
	header, inCtx := run(t, want.String())
	if header != want.String() || inCtx != want {
		t.Fatalf("header=%s ctx=%s, want %s", header, inCtx, want)
	}
}

func TestGeneratesIDWhenMissingOrInvalid(t *testing.T) {
	for _, incoming := range []string{"", "no-es-uuid", uuid.Nil.String()} {
		header, inCtx := run(t, incoming)
		if header == "" || header == incoming || inCtx.String() != header {
			t.Errorf("incoming %q: header=%q ctx=%s", incoming, header, inCtx)
		}
	}
}
