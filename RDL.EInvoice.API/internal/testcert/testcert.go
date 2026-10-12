// Package testcert genera, en cada prueba, certificados y .p12 de PRUEBA (autofirmados, sujeto «PRUEBA RDL»).
// Nunca se usa un certificado ni una llave de Hacienda en los tests (prompt P5): ni se guardan en el repo, porque
// se generan al vuelo. Solo lo importan archivos _test.go.
package testcert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	gopkcs12 "software.sslmate.com/src/go-pkcs12"
)

type Options struct {
	Bits      int
	NotBefore time.Time
	NotAfter  time.Time
	PIN       string
}

// P12 devuelve un PKCS#12 moderno (AES) con una llave RSA nueva y su certificado autofirmado.
func P12(t testing.TB, o Options) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, o.Bits)
	if err != nil {
		t.Fatal(err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 64))
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "PRUEBA RDL - CERTIFICADO DE TESTS", Country: []string{"CR"}, SerialNumber: "CPJ-3-101-000001"},
		NotBefore:    o.NotBefore,
		NotAfter:     o.NotAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	p12, err := gopkcs12.Modern.Encode(key, cert, nil, o.PIN)
	if err != nil {
		t.Fatal(err)
	}
	return p12
}
