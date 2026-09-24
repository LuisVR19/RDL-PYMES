package invitation

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"

	"rdl/platform-api/internal/domain/membership"
)

var now = time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

func TestNewStoresOnlyTheHash(t *testing.T) {
	inv, token, hash, err := New(uuid.New(), "  Ana@Example.TEST ", membership.RoleAccountant, uuid.New(), now)
	if err != nil {
		t.Fatal(err)
	}
	if inv.Email != "ana@example.test" || inv.Status != StatusPending || !inv.ExpiresAt.Equal(now.Add(TTL)) {
		t.Fatalf("inv=%+v", inv)
	}
	if !LooksLikeToken(token) || hash != HashToken(token) || hash == token {
		t.Fatal("token/hash inconsistentes")
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(hash) {
		t.Fatalf("el hash debe cumplir shared.sha256_hex: %q", hash)
	}
}

func TestTokensAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		_, token, _, _ := New(uuid.New(), "a@x.test", membership.RoleBiller, uuid.New(), now)
		if seen[token] {
			t.Fatal("token repetido")
		}
		seen[token] = true
	}
}

func TestNewValidates(t *testing.T) {
	if _, _, _, err := New(uuid.New(), "no-es-email", membership.RoleBiller, uuid.New(), now); !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("err=%v", err)
	}
	if _, _, _, err := New(uuid.New(), "a@x.test", "superadmin", uuid.New(), now); !errors.Is(err, membership.ErrInvalidRole) {
		t.Fatalf("err=%v", err)
	}
}

func TestCheckAcceptable(t *testing.T) {
	inv := Invitation{Email: "ana@example.test", Status: StatusPending, ExpiresAt: now.Add(time.Hour)}
	cases := []struct {
		name string
		inv  Invitation
		mail string
		at   time.Time
		want error
	}{
		{"vigente, mismo email (sin distinguir mayúsculas)", inv, "ANA@example.test", now, nil},
		{"otro email", inv, "beto@example.test", now, ErrEmailMismatch},
		{"vencida", inv, "ana@example.test", now.Add(2 * time.Hour), ErrExpired},
		{"justo al vencer", inv, "ana@example.test", now.Add(time.Hour), ErrExpired},
		{"ya aceptada", Invitation{Email: inv.Email, Status: StatusAccepted, ExpiresAt: inv.ExpiresAt}, "ana@example.test", now, ErrNotPending},
		{"revocada", Invitation{Email: inv.Email, Status: StatusRevoked, ExpiresAt: inv.ExpiresAt}, "ana@example.test", now, ErrNotPending},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.inv.CheckAcceptable(c.mail, c.at); !errors.Is(err, c.want) {
				t.Fatalf("err=%v, want %v", err, c.want)
			}
		})
	}
}

func TestLooksLikeToken(t *testing.T) {
	for _, s := range []string{"", "inv_", "abc", "inv_corto", "../../etc/passwd"} {
		if LooksLikeToken(s) {
			t.Errorf("%q no debería parecer un token", s)
		}
	}
}
