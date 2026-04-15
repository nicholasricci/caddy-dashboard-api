package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_RequiresJWTSecret(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, "configs")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	minimal := []byte("server:\n  port: \"8080\"\nauth:\n  token_ttl_minutes: 60\n")
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), minimal, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	t.Setenv("JWT_SECRET", "")
	t.Setenv("APP_AUTH_JWT_SECRET", "")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error when jwt secret is empty")
	}
}

func TestLoad_OKWithJWTSecret(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, "configs")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	minimal := []byte("server:\n  port: \"8080\"\nauth:\n  token_ttl_minutes: 60\n")
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), minimal, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	t.Setenv("JWT_SECRET", "unit-test-secret-at-least-32-chars-long")
	_, err := Load()
	if err != nil {
		t.Fatal(err)
	}
}
