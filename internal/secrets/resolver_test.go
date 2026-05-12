package secrets

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMultiResolver_FileAndEnv(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "k.txt")
	if err := os.WriteFile(keyPath, []byte("secret-bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := NewResolver(nil, 0)
	b, err := r.Resolve(context.Background(), "file://"+keyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "secret-bytes" {
		t.Fatalf("got %q", string(b))
	}

	t.Setenv("MCP_TEST_SECRET", "from-env")
	b2, err := r.Resolve(context.Background(), "env://MCP_TEST_SECRET")
	if err != nil {
		t.Fatal(err)
	}
	if string(b2) != "from-env" {
		t.Fatalf("got %q", string(b2))
	}
}
