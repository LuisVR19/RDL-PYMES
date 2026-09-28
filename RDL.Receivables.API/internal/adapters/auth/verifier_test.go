package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	testIssuer   = "https://proyecto.supabase.co/auth/v1"
	testAudience = "authenticated"
	testKID      = "test-kid"
)

func newKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// jwksFor publica la clave pública como lo hace Supabase (EC P-256, ES256).
func jwksFor(t *testing.T, pub *ecdsa.PublicKey) keyfunc.Keyfunc {
	t.Helper()
	point, err := pub.Bytes() // 0x04 || X || Y
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	set := map[string]any{"keys": []map[string]any{{
		"kty": "EC", "crv": "P-256", "alg": "ES256", "use": "sig", "kid": testKID,
		"x": enc(point[1:33]), "y": enc(point[33:]),
	}}}
	raw, _ := json.Marshal(set)
	kf, err := keyfunc.NewJWKSetJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	return kf
}

func validClaims() jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss":           testIssuer,
		"aud":           testAudience,
		"sub":           "8d0f5a52-5c1b-4c1e-9d38-2f6f0b0f6c11",
		"exp":           now.Add(time.Hour).Unix(),
		"iat":           now.Unix(),
		"role":          "authenticated",
		"email":         "ana@example.test",
		"is_anonymous":  false,
		"user_metadata": map[string]any{"full_name": "Ana Mora"},
	}
}

func sign(t *testing.T, key *ecdsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tok.Header["kid"] = testKID
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestVerifyValidTokenWithOrganization(t *testing.T) {
	key := newKey(t)
	v := newVerifier(jwksFor(t, &key.PublicKey), testIssuer, testAudience)
	org := uuid.New()
	c := validClaims()
	c["org_id"] = org.String()
	c["org_roles"] = []string{"owner"}

	id, err := v.Verify(t.Context(), sign(t, key, c))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if id.Subject != c["sub"] || id.Email != "ana@example.test" || id.FullName != "Ana Mora" || id.OrganizationID != org {
		t.Fatalf("identity = %+v", id)
	}
}

func TestVerifyWithoutOrgClaimHasNoOrganization(t *testing.T) {
	key := newKey(t)
	v := newVerifier(jwksFor(t, &key.PublicKey), testIssuer, testAudience)
	for _, org := range []any{nil, "", "no-es-uuid", uuid.Nil.String()} {
		c := validClaims()
		if org != nil {
			c["org_id"] = org
		}
		id, err := v.Verify(t.Context(), sign(t, key, c))
		if err != nil {
			t.Fatalf("org_id=%v: %v", org, err)
		}
		if id.HasOrganization() {
			t.Fatalf("org_id=%v no debería producir organización", org)
		}
	}
}

func TestVerifyRejectsInvalidTokens(t *testing.T) {
	key := newKey(t)
	other := newKey(t)
	v := newVerifier(jwksFor(t, &key.PublicKey), testIssuer, testAudience)

	with := func(k string, val any) jwt.MapClaims {
		c := validClaims()
		if val == nil {
			delete(c, k)
		} else {
			c[k] = val
		}
		return c
	}
	hs256 := func() string {
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims())
		tok.Header["kid"] = testKID
		s, _ := tok.SignedString([]byte("secreto-legado"))
		return s
	}

	cases := map[string]string{
		"expirado":                          sign(t, key, with("exp", time.Now().Add(-time.Hour).Unix())),
		"sin exp":                           sign(t, key, with("exp", nil)),
		"otra audiencia":                    sign(t, key, with("aud", "anon")),
		"otro emisor":                       sign(t, key, with("iss", "https://otro.supabase.co/auth/v1")),
		"sin sub":                           sign(t, key, with("sub", nil)),
		"sesión anónima":                    sign(t, key, with("is_anonymous", true)),
		"emitido en futuro":                 sign(t, key, with("iat", time.Now().Add(time.Hour).Unix())),
		"firmado con otra clave, mismo kid": sign(t, other, validClaims()),
		"HS256 con secreto compartido":      hs256(),
		"basura":                            "no.es.jwt",
		"vacío":                             "",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := v.Verify(t.Context(), raw); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("se esperaba ErrInvalidToken, got %v", err)
			}
		})
	}
}
