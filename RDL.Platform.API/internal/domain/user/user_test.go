package user

import (
	"errors"
	"testing"
)

func TestNewFromIdentity(t *testing.T) {
	u, err := NewFromIdentity("sub", " ana@example.test ", " Ana Mora ")
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "ana@example.test" || u.FullName != "Ana Mora" || u.Status != StatusActive || u.Subject != "sub" {
		t.Fatalf("u=%+v", u)
	}
}

func TestNewFromIdentityFallsBackToEmailLocalPart(t *testing.T) {
	u, err := NewFromIdentity("sub", "carlos.solis@example.test", "")
	if err != nil {
		t.Fatal(err)
	}
	if u.FullName != "carlos.solis" {
		t.Fatalf("fullName=%q", u.FullName)
	}
}

func TestNewFromIdentityRequiresValidEmail(t *testing.T) {
	for _, email := range []string{"", "no-es-email", "Ana <ana@example.test>"} {
		if _, err := NewFromIdentity("sub", email, "Ana"); !errors.Is(err, ErrEmailRequired) {
			t.Errorf("email %q: err=%v", email, err)
		}
	}
}
