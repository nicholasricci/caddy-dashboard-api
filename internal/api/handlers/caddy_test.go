package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	caddysvc "github.com/nicholasricci/caddy-dashboard/internal/caddy"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
)

type fakeCaddyService struct {
	listIDs       []models.CaddyConfigIDInfo
	listIDsErr    error
	configByID    json.RawMessage
	configByIDErr error
	upstreams     []json.RawMessage
	upstreamsErr  error
	hosts         []string
	hostsErr      error
}

func (f *fakeCaddyService) Sync(context.Context, uuid.UUID, string) error { return nil }

func (f *fakeCaddyService) GetLiveConfig(context.Context, uuid.UUID) (json.RawMessage, error) {
	return nil, nil
}

func (f *fakeCaddyService) ListConfigIDs(context.Context, uuid.UUID) ([]models.CaddyConfigIDInfo, error) {
	return f.listIDs, f.listIDsErr
}

func (f *fakeCaddyService) GetConfigByID(context.Context, uuid.UUID, string) (json.RawMessage, error) {
	return f.configByID, f.configByIDErr
}

func (f *fakeCaddyService) GetUpstreamsByID(context.Context, uuid.UUID, string) ([]json.RawMessage, error) {
	return f.upstreams, f.upstreamsErr
}

func (f *fakeCaddyService) GetHostsByID(context.Context, uuid.UUID, string) ([]string, error) {
	return f.hosts, f.hostsErr
}

func (f *fakeCaddyService) Apply(context.Context, uuid.UUID, json.RawMessage, string) error {
	return nil
}

func (f *fakeCaddyService) Reload(context.Context, uuid.UUID) error { return nil }

func (f *fakeCaddyService) ListSnapshotsPaginated(context.Context, uuid.UUID, int, int) ([]models.CaddySnapshot, int64, error) {
	return nil, 0, nil
}

func TestCaddyHandler_ListConfigIDs_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	nodeID := uuid.New()
	handler := NewCaddyHandler(&fakeCaddyService{
		listIDs: []models.CaddyConfigIDInfo{
			{ID: "route-a", HasUpstreams: true, UpstreamCount: 1, Upstreams: []any{map[string]any{"dial": "10.0.0.1:8080"}}},
		},
	}, nil)

	r := gin.New()
	r.GET("/nodes/:id/config/live/ids", handler.ListConfigIDs)
	req := httptest.NewRequest(http.MethodGet, "/nodes/"+nodeID.String()+"/config/live/ids", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestCaddyHandler_ConfigByID_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	nodeID := uuid.New()
	handler := NewCaddyHandler(&fakeCaddyService{
		configByIDErr: caddysvc.ErrConfigIDNotFound,
	}, nil)

	r := gin.New()
	r.GET("/nodes/:id/config/live/ids/:configId", handler.ConfigByID)
	req := httptest.NewRequest(http.MethodGet, "/nodes/"+nodeID.String()+"/config/live/ids/missing", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want %d body=%s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

func TestCaddyHandler_UpstreamsByID_InternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	nodeID := uuid.New()
	handler := NewCaddyHandler(&fakeCaddyService{
		upstreamsErr: errors.New("boom"),
	}, nil)

	r := gin.New()
	r.GET("/nodes/:id/config/live/ids/:configId/upstreams", handler.UpstreamsByID)
	req := httptest.NewRequest(http.MethodGet, "/nodes/"+nodeID.String()+"/config/live/ids/route-a/upstreams", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want %d body=%s", w.Code, http.StatusInternalServerError, w.Body.String())
	}
}

func TestCaddyHandler_HostsByID_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	nodeID := uuid.New()
	handler := NewCaddyHandler(&fakeCaddyService{
		hosts: []string{"10.0.0.1", "api.internal"},
	}, nil)

	r := gin.New()
	r.GET("/nodes/:id/config/live/ids/:configId/hosts", handler.HostsByID)
	req := httptest.NewRequest(http.MethodGet, "/nodes/"+nodeID.String()+"/config/live/ids/route-a/hosts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d body=%s", w.Code, http.StatusOK, w.Body.String())
	}
}
