// Package pkcs12 abre el archivo .p12 de la organización (la llave criptográfica de Hacienda viene así) para
// validarlo al subirlo y, en el incremento 8, para firmar. La llave privada solo vive en memoria mientras se usa.
package pkcs12

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"math/big"
	"strings"

	gopkcs12 "software.sslmate.com/src/go-pkcs12"

	"rdl/einvoice-api/internal/domain/certificate"
)

// MaxFileBytes acota el .p12 aceptado: una llave RSA 4096 con su cadena cabe de sobra en 64 KiB.
const MaxFileBytes = 64 << 10

// Reader implementa app.CertificateReader.
type Reader struct{}

// Inspect abre el PKCS#12 con el PIN y devuelve los datos públicos del certificado. Ningún error incluye el PIN ni
// el contenido del archivo, y un PIN incorrecto no se distingue de un archivo inválido.
func (Reader) Inspect(p12 []byte, pin string) (certificate.Info, error) {
	if len(p12) == 0 || len(p12) > MaxFileBytes {
		return certificate.Info{}, certificate.ErrUnreadable
	}
	key, cert, _, err := gopkcs12.DecodeChain(p12, pin)
	if err != nil {
		return certificate.Info{}, certificate.ErrUnreadable
	}
	info := certificate.Info{
		Subject:      cert.Subject.String(),
		Issuer:       cert.Issuer.String(),
		SerialNumber: serial(cert.SerialNumber),
		NotBefore:    cert.NotBefore.UTC(),
		NotAfter:     cert.NotAfter.UTC(),
		KeyAlgorithm: algorithm(cert),
	}
	if pub, ok := cert.PublicKey.(*rsa.PublicKey); ok {
		info.KeyBits = pub.N.BitLen()
		// La llave privada tiene que ser la del certificado: si no, la firma no verificaría.
		priv, ok := key.(*rsa.PrivateKey)
		if !ok || priv.N.Cmp(pub.N) != 0 {
			return certificate.Info{}, fmt.Errorf("%w: la llave privada no corresponde al certificado", certificate.ErrUnreadable)
		}
	}
	return info, nil
}

func algorithm(c *x509.Certificate) string {
	switch c.PublicKeyAlgorithm {
	case x509.RSA:
		return "RSA"
	case x509.ECDSA:
		return "ECDSA"
	case x509.Ed25519:
		return "Ed25519"
	default:
		return strings.ToUpper(c.PublicKeyAlgorithm.String())
	}
}

func serial(n *big.Int) string {
	if n == nil {
		return ""
	}
	return n.String() // decimal, como X509SerialNumber en el ejemplo del Anexo 2 (pág. 88)
}
