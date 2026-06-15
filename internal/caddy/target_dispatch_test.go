package caddy

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nicholasricci/caddy-dashboard/internal/models"
)

func TestBuildExecTarget_GCPAzure(t *testing.T) {
	gcpCfg, _ := json.Marshal(map[string]any{
		"project_id": "p1", "zone": "z1", "instance_name": "vm1",
		"label_key": "env", "label_value": "prod",
	})
	nGCP := &models.CaddyNode{
		Transport:       models.TransportGCPOsConfig,
		TransportConfig: gcpCfg,
	}
	got, err := BuildExecTarget(nGCP)
	if err != nil {
		t.Fatal(err)
	}
	if got.GCP == nil || got.GCP.ProjectID != "p1" || got.GCP.LabelKey != "env" {
		t.Fatalf("GCP target: %+v", got.GCP)
	}

	azCfg, _ := json.Marshal(map[string]any{
		"subscription_id": "sub", "resource_group": "rg", "vm_name": "vm2",
	})
	nAz := &models.CaddyNode{
		Transport:       models.TransportAzureRunCommand,
		TransportConfig: azCfg,
	}
	got2, err := BuildExecTarget(nAz)
	if err != nil {
		t.Fatal(err)
	}
	if got2.Azure == nil || got2.Azure.VMName != "vm2" {
		t.Fatalf("Azure target: %+v", got2.Azure)
	}
}

func TestDispatcher_GCPOsConfigUsesExecutor(t *testing.T) {
	f := &fakeExecutor{
		fetchResult: ExecutionResult{Status: ExecStatusSuccess, Stdout: "{}"},
	}
	d := NewDispatcher(map[string]RemoteExecutor{
		models.TransportGCPOsConfig: f,
	})
	target := ExecTarget{
		Node:      &models.CaddyNode{Transport: models.TransportGCPOsConfig},
		Transport: models.TransportGCPOsConfig,
	}
	_, err := d.FetchConfig(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if f.fetchCalls != 1 {
		t.Fatalf("fetchCalls=%d", f.fetchCalls)
	}
}
