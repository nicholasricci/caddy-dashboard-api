package auth

import (
	"testing"

	"github.com/nicholasricci/caddy-dashboard/internal/models"
)

func TestJWTManager_GeneratePair_AccessVsRefresh(t *testing.T) {
	m := NewJWTManager("test-secret-key-min-32-bytes-ok", 15, 60*24, "test-issuer", "test-audience")
	access, refresh, err := m.GeneratePair("alice", models.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if access == "" || refresh == "" {
		t.Fatal("expected non-empty tokens")
	}

	accClaims, err := m.ValidateAccessToken(access)
	if err != nil {
		t.Fatalf("access token invalid: %v", err)
	}
	if accClaims.Username != "alice" || accClaims.Role != models.RoleAdmin {
		t.Fatalf("claims: %+v", accClaims)
	}

	if _, err := m.ValidateAccessToken(refresh); err == nil {
		t.Fatal("refresh token must not validate as access")
	}

	refClaims, err := m.ValidateRefreshToken(refresh)
	if err != nil {
		t.Fatalf("refresh token invalid: %v", err)
	}
	if refClaims.TokenUse != "refresh" {
		t.Fatalf("token_use: %q", refClaims.TokenUse)
	}

	if _, err := m.ValidateRefreshToken(access); err == nil {
		t.Fatal("access token must not validate as refresh")
	}
}
