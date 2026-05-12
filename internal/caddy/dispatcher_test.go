package caddy

import (
	"context"
	"errors"
	"testing"

	"github.com/nicholasricci/caddy-dashboard/internal/models"
)

func TestDispatcher_InventoryOnly(t *testing.T) {
	d := NewDispatcher(map[string]RemoteExecutor{
		models.TransportAWSSSM: &fakeExecutor{},
	})
	target := ExecTarget{
		Node:      &models.CaddyNode{Transport: models.TransportInventoryOnly},
		Transport: models.TransportInventoryOnly,
	}
	_, err := d.FetchConfig(context.Background(), target)
	if !errors.Is(err, ErrTransportUnsupportedOp) {
		t.Fatalf("got %v, want ErrTransportUnsupportedOp", err)
	}
}

func TestDispatcher_UnknownTransport(t *testing.T) {
	d := NewDispatcher(map[string]RemoteExecutor{})
	_, err := d.FetchConfig(context.Background(), ExecTarget{
		Node:      &models.CaddyNode{Transport: "nope"},
		Transport: "nope",
	})
	if !errors.Is(err, ErrUnknownTransport) {
		t.Fatalf("got %v, want ErrUnknownTransport", err)
	}
}
