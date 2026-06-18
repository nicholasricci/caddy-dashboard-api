package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/nicholasricci/caddy-dashboard/internal/models"
)

const apiKeyRandomBytes = 32

// HashAPIKey returns the SHA-256 hex digest of a plaintext API key.
func HashAPIKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// GenerateAPIKeySecret creates a new plaintext secret, its hash, and a display prefix.
func GenerateAPIKeySecret() (plaintext, hash, prefix string, err error) {
	buf := make([]byte, apiKeyRandomBytes)
	if _, err = rand.Read(buf); err != nil {
		return "", "", "", fmt.Errorf("generate api key: %w", err)
	}
	plaintext = models.APIKeyPrefix + base64.RawURLEncoding.EncodeToString(buf)
	hash = HashAPIKey(plaintext)
	prefix = keyDisplayPrefix(plaintext)
	return plaintext, hash, prefix, nil
}

func keyDisplayPrefix(plaintext string) string {
	plain := strings.TrimSpace(plaintext)
	if len(plain) <= 16 {
		return plain
	}
	return plain[:16] + "..."
}

// IsAPIKeyToken reports whether a bearer token looks like an API key (vs JWT).
func IsAPIKeyToken(token string) bool {
	return strings.HasPrefix(strings.TrimSpace(token), models.APIKeyPrefix)
}
