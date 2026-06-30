package tasks

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
	"github.com/nicholasricci/caddy-dashboard/internal/repository/testutil"
)

func TestParseUpstreamHealthcheckConfig(t *testing.T) {
	validID := "8040ce98-d808-4d3d-b2c4-f6161144188c"

	tests := []struct {
		name    string
		raw     json.RawMessage
		wantErr string
	}{
		{
			name:    "missing config",
			raw:     nil,
			wantErr: "missing upstream_healthcheck config",
		},
		{
			name:    "null config",
			raw:     json.RawMessage("null"),
			wantErr: "missing upstream_healthcheck config",
		},
		{
			name:    "empty object",
			raw:     json.RawMessage("{}"),
			wantErr: "missing discovery_config_id",
		},
		{
			name:    "invalid json",
			raw:     json.RawMessage("{"),
			wantErr: "invalid upstream_healthcheck config",
		},
		{
			name:    "invalid discovery_config_id",
			raw:     json.RawMessage(`{"discovery_config_id":"not-a-uuid"}`),
			wantErr: "invalid discovery_config_id",
		},
		{
			name: "valid config",
			raw: json.RawMessage(`{
				"discovery_config_id":"` + validID + `",
				"config_ids":["https-made-in-italy-api"]
			}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, id, err := parseUpstreamHealthcheckConfig(tt.raw)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error=%q, want substring %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if id.String() != validID {
				t.Fatalf("id=%s, want %s", id, validID)
			}
			if len(cfg.ConfigIDs) != 1 || cfg.ConfigIDs[0] != "https-made-in-italy-api" {
				t.Fatalf("config_ids=%v, want [https-made-in-italy-api]", cfg.ConfigIDs)
			}
		})
	}
}

func TestUpstreamHealthcheckRun_DiscoveryConfigNotFound(t *testing.T) {
	db := testutil.NewTestDB(t)
	discoveryRepo := repository.NewDiscoveryRepository(db)
	task := &upstreamHealthcheckTask{discoveryRepo: discoveryRepo}

	missingID := uuid.New()
	raw := json.RawMessage(`{"discovery_config_id":"` + missingID.String() + `"}`)
	result, err := task.Run(context.Background(), raw)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("status=%q, want failed", result.Status)
	}
	if result.Error != "discovery config not found" {
		t.Fatalf("error=%q, want discovery config not found", result.Error)
	}
}
