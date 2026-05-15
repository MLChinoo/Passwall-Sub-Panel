package mysql

import (
	"crypto/sha256"
	"fmt"
	"strings"

	pspcrypto "github.com/KazuhaHub/passwall-sub-panel/internal/pkg/crypto"
)

const secretPrefix = "enc:v1:"

var dbSecretKey []byte

// ConfigureSecretKey installs the process-wide key material used by MySQL
// repositories to encrypt sensitive string fields before saving them.
func ConfigureSecretKey(material string) {
	material = strings.TrimSpace(material)
	if material == "" {
		dbSecretKey = nil
		return
	}
	sum := sha256.Sum256([]byte(material))
	key := make([]byte, len(sum))
	copy(key, sum[:])
	dbSecretKey = key
}

func encryptSecret(plaintext string) (string, error) {
	if plaintext == "" || strings.HasPrefix(plaintext, secretPrefix) {
		return plaintext, nil
	}
	if len(dbSecretKey) == 0 {
		return plaintext, nil
	}
	ciphertext, err := pspcrypto.EncryptString(dbSecretKey, plaintext)
	if err != nil {
		return "", err
	}
	return secretPrefix + ciphertext, nil
}

func decryptSecret(stored string) (string, error) {
	if stored == "" || !strings.HasPrefix(stored, secretPrefix) {
		return stored, nil
	}
	if len(dbSecretKey) == 0 {
		return "", fmt.Errorf("database secret is encrypted but no encryption key is configured")
	}
	plaintext, err := pspcrypto.DecryptString(dbSecretKey, strings.TrimPrefix(stored, secretPrefix))
	if err != nil {
		return "", fmt.Errorf("decrypt database secret: %w", err)
	}
	return plaintext, nil
}
