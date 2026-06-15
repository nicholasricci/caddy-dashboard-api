package services

import (
	"encoding/json"
	"testing"

	"github.com/nicholasricci/caddy-dashboard/internal/models"
)

func TestValidateCaddyNode_GCPAzure(t *testing.T) {
	goodGCP := &models.CaddyNode{
		Transport: models.TransportGCPOsConfig,
		TransportConfig: mustJSON(t, map[string]any{
			"project_id": "p", "zone": "z", "instance_name": "vm",
			"label_key": "k", "label_value": "v",
		}),
	}
	if err := ValidateCaddyNode(goodGCP); err != nil {
		t.Fatal(err)
	}
	badGCP := &models.CaddyNode{
		Transport: models.TransportGCPOsConfig,
		TransportConfig: mustJSON(t, map[string]any{
			"project_id": "p",
		}),
	}
	if err := ValidateCaddyNode(badGCP); err == nil {
		t.Fatal("expected error")
	}

	goodAz := &models.CaddyNode{
		Transport: models.TransportAzureRunCommand,
		TransportConfig: mustJSON(t, map[string]any{
			"subscription_id": "s", "resource_group": "rg", "vm_name": "vm",
		}),
	}
	if err := ValidateCaddyNode(goodAz); err != nil {
		t.Fatal(err)
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return json.RawMessage(b)
}
