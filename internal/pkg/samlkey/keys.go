// Package samlkey generates self-signed SP keypairs for the SAML
// "auto" mode. The cert is only used to sign AuthnRequests and to
// satisfy crewjam/saml's requirement that a ServiceProvider have a
// certificate — IdPs don't typically validate it against a CA chain
// for SP-initiated SSO. RSA-2048 / 10-year validity is sufficient.
package samlkey

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

// GenerateSelfSigned returns matching cert + key PEMs suitable for the
// SAML SP. commonName is the SP entity ID; if empty, a placeholder is used.
func GenerateSelfSigned(commonName string) (certPEM, keyPEM string, err error) {
	if commonName == "" {
		commonName = "passwall-sub-panel-sp"
	}
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("rsa generate: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", fmt.Errorf("serial: %w", err)
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             now.Add(-1 * time.Hour),
		NotAfter:              now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return "", "", fmt.Errorf("create cert: %w", err)
	}
	certBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})
	return string(certBytes), string(keyBytes), nil
}
