package caddy

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

type fakeNodeLoader struct {
	node *models.CaddyNode
	err  error
}

func (f *fakeNodeLoader) GetByID(_ context.Context, _ uuid.UUID) (*models.CaddyNode, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.node, nil
}

type fakeDiscoveryLoader struct {
	cfg *models.DiscoveryConfig
	err error
}

func (f *fakeDiscoveryLoader) GetByID(_ context.Context, _ uuid.UUID) (*models.DiscoveryConfig, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.cfg, nil
}

type fakeSnapshotWriter struct {
	last *models.CaddySnapshot
}

func (f *fakeSnapshotWriter) Create(_ context.Context, s *models.CaddySnapshot) error {
	dup := *s
	f.last = &dup
	return nil
}

func TestService_GetLiveConfig_NodeNotFound(t *testing.T) {
	svc := NewService(&fakeNodeLoader{err: gorm.ErrRecordNotFound}, &fakeDiscoveryLoader{}, nil, nil)
	_, err := svc.GetLiveConfig(context.Background(), uuid.New())
	if !errors.Is(err, models.ErrNodeNotFound) {
		t.Fatalf("GetLiveConfig: got %v, want models.ErrNodeNotFound", err)
	}
}

func TestService_GetLiveConfig_NoInstanceID(t *testing.T) {
	node := &models.CaddyNode{
		ID:         uuid.New(),
		Name:       "n",
		Region:     "eu-west-1",
		InstanceID: nil,
	}
	svc := NewService(&fakeNodeLoader{node: node}, &fakeDiscoveryLoader{}, nil, nil)
	_, err := svc.GetLiveConfig(context.Background(), node.ID)
	if !errors.Is(err, ErrNodeNoInstanceID) {
		t.Fatalf("GetLiveConfig: got %v, want ErrNodeNoInstanceID", err)
	}
}

func TestService_storeSnapshot_NodeScope(t *testing.T) {
	nodeID := uuid.New()
	discoveryID := uuid.New()
	writer := &fakeSnapshotWriter{}
	svc := NewService(
		&fakeNodeLoader{},
		&fakeDiscoveryLoader{cfg: &models.DiscoveryConfig{ID: discoveryID, SnapshotScope: models.SnapshotScopeNode}},
		writer,
		nil,
	)
	node := &models.CaddyNode{ID: nodeID, DiscoveryConfigID: &discoveryID}
	payload, _ := json.Marshal(map[string]any{"apps": map[string]any{}})

	if err := svc.storeSnapshot(context.Background(), node, payload, "tester"); err != nil {
		t.Fatalf("storeSnapshot: unexpected error: %v", err)
	}
	if writer.last == nil || writer.last.NodeID == nil || *writer.last.NodeID != nodeID {
		t.Fatalf("NodeID=%v, want %s", writer.last.NodeID, nodeID)
	}
	if writer.last.DiscoveryConfigID != nil {
		t.Fatalf("DiscoveryConfigID=%v, want nil for node scope (should not pollute group listings)", writer.last.DiscoveryConfigID)
	}
}

func TestService_storeSnapshot_NoDiscoveryConfig(t *testing.T) {
	nodeID := uuid.New()
	writer := &fakeSnapshotWriter{}
	svc := NewService(
		&fakeNodeLoader{},
		&fakeDiscoveryLoader{},
		writer,
		nil,
	)
	node := &models.CaddyNode{ID: nodeID}
	payload, _ := json.Marshal(map[string]any{"apps": map[string]any{}})

	if err := svc.storeSnapshot(context.Background(), node, payload, "tester"); err != nil {
		t.Fatalf("storeSnapshot: unexpected error: %v", err)
	}
	if writer.last == nil || writer.last.NodeID == nil || *writer.last.NodeID != nodeID {
		t.Fatalf("NodeID=%v, want %s", writer.last.NodeID, nodeID)
	}
	if writer.last.DiscoveryConfigID != nil {
		t.Fatalf("DiscoveryConfigID=%v, want nil when node has no discovery", writer.last.DiscoveryConfigID)
	}
}

func TestService_storeSnapshot_DiscoveryConfigNotFoundFallsBackToNodeScope(t *testing.T) {
	nodeID := uuid.New()
	discoveryID := uuid.New()
	writer := &fakeSnapshotWriter{}
	svc := NewService(
		&fakeNodeLoader{},
		&fakeDiscoveryLoader{err: gorm.ErrRecordNotFound},
		writer,
		nil,
	)
	node := &models.CaddyNode{ID: nodeID, DiscoveryConfigID: &discoveryID}
	payload, _ := json.Marshal(map[string]any{"apps": map[string]any{}})

	if err := svc.storeSnapshot(context.Background(), node, payload, "tester"); err != nil {
		t.Fatalf("storeSnapshot: unexpected error: %v", err)
	}
	if writer.last == nil || writer.last.NodeID == nil || *writer.last.NodeID != nodeID {
		t.Fatalf("NodeID=%v, want %s", writer.last.NodeID, nodeID)
	}
	if writer.last.DiscoveryConfigID != nil {
		t.Fatalf("DiscoveryConfigID=%v, want nil after NotFound fallback", writer.last.DiscoveryConfigID)
	}
}

func TestService_storeSnapshot_DiscoveryLookupErrorPropagates(t *testing.T) {
	discoveryID := uuid.New()
	want := errors.New("db down")
	svc := NewService(
		&fakeNodeLoader{},
		&fakeDiscoveryLoader{err: want},
		&fakeSnapshotWriter{},
		nil,
	)
	node := &models.CaddyNode{ID: uuid.New(), DiscoveryConfigID: &discoveryID}
	payload, _ := json.Marshal(map[string]any{"apps": map[string]any{}})

	err := svc.storeSnapshot(context.Background(), node, payload, "tester")
	if !errors.Is(err, want) {
		t.Fatalf("storeSnapshot: got %v, want wrapping of %v", err, want)
	}
}

func TestService_storeSnapshot_GroupScope(t *testing.T) {
	nodeID := uuid.New()
	discoveryID := uuid.New()
	writer := &fakeSnapshotWriter{}
	svc := NewService(
		&fakeNodeLoader{},
		&fakeDiscoveryLoader{cfg: &models.DiscoveryConfig{ID: discoveryID, SnapshotScope: models.SnapshotScopeGroup}},
		writer,
		nil,
	)
	node := &models.CaddyNode{ID: nodeID, DiscoveryConfigID: &discoveryID}
	payload, _ := json.Marshal(map[string]any{"admin": map[string]any{}})

	if err := svc.storeSnapshot(context.Background(), node, payload, "tester"); err != nil {
		t.Fatalf("storeSnapshot: unexpected error: %v", err)
	}
	if writer.last == nil {
		t.Fatal("expected snapshot to be created")
	}
	if writer.last.NodeID != nil {
		t.Fatalf("NodeID=%v, want nil for group scope", writer.last.NodeID)
	}
	if writer.last.DiscoveryConfigID == nil || *writer.last.DiscoveryConfigID != discoveryID {
		t.Fatalf("DiscoveryConfigID=%v, want %s", writer.last.DiscoveryConfigID, discoveryID)
	}
}
