package auth_test

import (
	"strings"
	"testing"

	"github.com/nicholasricci/caddy-dashboard/internal/auth"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
)

func TestGenerateAPIKeySecret(t *testing.T) {
	plain, hash, prefix, err := auth.GenerateAPIKeySecret()
	if err != nil {
		t.Fatalf("GenerateAPIKeySecret: %v", err)
	}
	if !strings.HasPrefix(plain, models.APIKeyPrefix) {
		t.Fatalf("prefix missing: %q", plain)
	}
	if hash != auth.HashAPIKey(plain) {
		t.Fatal("hash mismatch")
	}
	if prefix == "" {
		t.Fatal("empty display prefix")
	}
}

func TestIsAPIKeyToken(t *testing.T) {
	if !auth.IsAPIKeyToken(models.APIKeyPrefix + "abc") {
		t.Fatal("expected api key token")
	}
	if auth.IsAPIKeyToken("eyJhbGciOiJIUzI1NiJ9") {
		t.Fatal("jwt should not match api key")
	}
}
