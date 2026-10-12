package secrets

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func newStore(t *testing.T) *FileStore {
	t.Helper()
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	s, err := NewFileStore(t.TempDir(), base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRoundTripAndNoPlaintextOnDisk(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	org := uuid.New()
	secret := []byte("PIN-1234-no-debe-verse")
	id, err := s.Put(ctx, org, secret)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, org, id)
	if err != nil || !bytes.Equal(got, secret) {
		t.Fatalf("Get = %q, %v", got, err)
	}
	raw, err := os.ReadFile(filepath.Join(s.dir, org.String(), id.String()+".bin"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, secret) || bytes.Contains(raw, []byte("1234")) {
		t.Fatal("el archivo guarda el secreto en claro")
	}
	if err := s.Delete(ctx, org, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, org, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("después de borrar: %v", err)
	}
}

// Un secreto de A no se abre como de B, ni copiando el archivo a la carpeta de B.
func TestSecretIsBoundToItsOrganization(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	orgA, orgB := uuid.New(), uuid.New()
	id, err := s.Put(ctx, orgA, []byte("llave privada de A"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, orgB, id); err == nil {
		t.Fatal("B leyó un secreto de A")
	}
	raw, _ := os.ReadFile(filepath.Join(s.dir, orgA.String(), id.String()+".bin"))
	_ = os.MkdirAll(filepath.Join(s.dir, orgB.String()), 0o700)
	_ = os.WriteFile(filepath.Join(s.dir, orgB.String(), id.String()+".bin"), raw, 0o600) // #nosec G703 -- directorio temporal de la prueba
	if _, err := s.Get(ctx, orgB, id); err == nil {
		t.Fatal("el archivo de A copiado a la carpeta de B se descifró")
	}
}

func TestRejectsBadMasterKey(t *testing.T) {
	for _, k := range []string{"", "no-es-base64!", base64.StdEncoding.EncodeToString(make([]byte, 16))} {
		if _, err := NewFileStore(t.TempDir(), k); err == nil {
			t.Errorf("llave %q aceptada", k)
		}
	}
}
