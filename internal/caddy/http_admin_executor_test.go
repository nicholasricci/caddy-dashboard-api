package caddy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/secrets"
)

func TestHTTPAdminExecutor_FetchConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/config/" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"apps":{}}`))
	}))
	defer srv.Close()

	ex := NewHTTPAdminExecutor(secrets.NewResolver(nil, 0), 0, 1024)
	cfgBytes, err := json.Marshal(map[string]string{"base_url": srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	target, err := BuildExecTarget(&models.CaddyNode{
		Transport:       models.TransportHTTPAdmin,
		TransportConfig: cfgBytes,
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := ex.FetchConfig(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != ExecStatusSuccess {
		t.Fatalf("status=%s stderr=%s", res.Status, res.Stderr)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &parsed); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPAdminExecutor_ReloadUnsupported(t *testing.T) {
	ex := NewHTTPAdminExecutor(secrets.NewResolver(nil, 0), 0, 1024)
	target, err := BuildExecTarget(&models.CaddyNode{
		Transport:       models.TransportHTTPAdmin,
		TransportConfig: []byte(`{"base_url": "http://127.0.0.1:2019"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = ex.Reload(context.Background(), target)
	if err != ErrTransportUnsupportedOp {
		t.Fatalf("got %v, want ErrTransportUnsupportedOp", err)
	}
}
