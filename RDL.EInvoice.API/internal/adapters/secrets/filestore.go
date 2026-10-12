// Package secrets implementa app.SecretStore. Hoy solo el adapter de desarrollo (decisión 6): archivos cifrados con
// AES-256-GCM en un directorio ignorado por git. TODO(P2): el gestor de secretos de la plataforma.
package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// ErrNotFound: no hay un secreto con ese id para esa organización.
var ErrNotFound = errors.New("secreto no encontrado")

// FileStore guarda cada secreto en <dir>/<organización>/<id>.bin con el formato nonce || texto cifrado.
//
// La organización y el id van como datos asociados (AAD) del cifrado: un secreto de A no se descifra pidiéndolo como
// de B aunque alguien copie el archivo de carpeta. Así «el certificado de A nunca firma un documento de B» también
// lo garantiza el almacenamiento, no solo la consulta.
type FileStore struct {
	dir  string
	aead cipher.AEAD
}

// NewFileStore recibe la llave maestra en base64 (32 bytes, AES-256). Nunca se registra en logs.
func NewFileStore(dir, masterKeyB64 string) (*FileStore, error) {
	key, err := base64.StdEncoding.DecodeString(masterKeyB64)
	if err != nil || len(key) != 32 {
		return nil, errors.New("SECRETS_MASTER_KEY debe ser 32 bytes en base64")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("creando el directorio de secretos: %w", err)
	}
	return &FileStore{dir: dir, aead: aead}, nil
}

func (s *FileStore) path(org, id uuid.UUID) string {
	return filepath.Join(s.dir, org.String(), id.String()+".bin")
}

func aad(org, id uuid.UUID) []byte {
	return []byte("rdl-fiscal-secret/" + org.String() + "/" + id.String())
}

func (s *FileStore) Put(_ context.Context, org uuid.UUID, data []byte) (uuid.UUID, error) {
	if org == uuid.Nil {
		return uuid.Nil, errors.New("secreto sin organización")
	}
	id := uuid.New()
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return uuid.Nil, err
	}
	sealed := s.aead.Seal(nonce, nonce, data, aad(org, id))
	if err := os.MkdirAll(filepath.Join(s.dir, org.String()), 0o700); err != nil {
		return uuid.Nil, fmt.Errorf("guardando el secreto: %w", err)
	}
	// O_EXCL: un id nuevo nunca pisa otro archivo.
	f, err := os.OpenFile(s.path(org, id), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- ruta armada con uuids
	if err != nil {
		return uuid.Nil, fmt.Errorf("guardando el secreto: %w", err)
	}
	if _, err := f.Write(sealed); err != nil {
		_ = f.Close()
		_ = os.Remove(s.path(org, id))
		return uuid.Nil, fmt.Errorf("guardando el secreto: %w", err)
	}
	if err := f.Close(); err != nil {
		return uuid.Nil, fmt.Errorf("guardando el secreto: %w", err)
	}
	return id, nil
}

func (s *FileStore) Get(_ context.Context, org, id uuid.UUID) ([]byte, error) {
	raw, err := os.ReadFile(s.path(org, id)) // #nosec G304 -- ruta armada con uuids
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("leyendo el secreto: %w", err)
	}
	n := s.aead.NonceSize()
	if len(raw) < n {
		return nil, errors.New("secreto dañado")
	}
	plain, err := s.aead.Open(nil, raw[:n], raw[n:], aad(org, id))
	if err != nil {
		// Otra organización, otra llave maestra o un archivo alterado: no se distingue.
		return nil, errors.New("el secreto no se pudo descifrar")
	}
	return plain, nil
}

func (s *FileStore) Delete(_ context.Context, org, id uuid.UUID) error {
	err := os.Remove(s.path(org, id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
