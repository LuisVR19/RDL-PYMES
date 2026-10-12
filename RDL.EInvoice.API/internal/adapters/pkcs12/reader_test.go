package pkcs12

import (
	"errors"
	"strings"
	"testing"
	"time"

	"rdl/einvoice-api/internal/domain/certificate"
	"rdl/einvoice-api/internal/testcert"
)

func TestInspect(t *testing.T) {
	now := time.Now()
	p12 := testcert.P12(t, testcert.Options{Bits: 2048, NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(1, 0, 0), PIN: "1234"})
	info, err := Reader{}.Inspect(p12, "1234")
	if err != nil {
		t.Fatal(err)
	}
	if info.KeyAlgorithm != "RSA" || info.KeyBits != 2048 || info.SerialNumber == "" || !strings.Contains(info.Subject, "PRUEBA") {
		t.Errorf("info = %+v", info)
	}
	if err := certificate.Validate(info, now); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestWrongPINOrGarbageLookTheSame(t *testing.T) {
	now := time.Now()
	p12 := testcert.P12(t, testcert.Options{Bits: 2048, NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(1, 0, 0), PIN: "1234"})
	for name, c := range map[string]struct {
		data []byte
		pin  string
	}{
		"PIN incorrecto":   {p12, "9999"},
		"no es un p12":     {[]byte("hola"), "1234"},
		"vacío":            {nil, "1234"},
		"demasiado grande": {make([]byte, MaxFileBytes+1), "1234"},
	} {
		_, err := Reader{}.Inspect(c.data, c.pin)
		if !errors.Is(err, certificate.ErrUnreadable) {
			t.Errorf("%s: err = %v", name, err)
		}
		if err != nil && (strings.Contains(err.Error(), "9999") || strings.Contains(err.Error(), "1234")) {
			t.Errorf("%s: el error menciona el PIN: %v", name, err)
		}
	}
}
