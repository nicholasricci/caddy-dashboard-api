package caddy

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	awssvc "github.com/nicholasricci/caddy-dashboard/internal/aws"
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

type fakeExecutor struct {
	fetchResult awssvc.CommandResult
	fetchErr    error
	fetchCalls  int
}

func (f *fakeExecutor) ApplyConfig(_ context.Context, _, _ string, _ []byte) (*awssvc.CommandResult, error) {
	return &awssvc.CommandResult{Status: "Success"}, nil
}

func (f *fakeExecutor) Reload(_ context.Context, _, _ string) (*awssvc.CommandResult, error) {
	return &awssvc.CommandResult{Status: "Success"}, nil
}

func (f *fakeExecutor) FetchConfig(_ context.Context, _, _ string) (*awssvc.CommandResult, error) {
	f.fetchCalls++
	if f.fetchErr != nil {
		return nil, f.fetchErr
	}
	result := f.fetchResult
	return &result, nil
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

func TestService_ListConfigIDs_AndLookupByID(t *testing.T) {
	instanceID := "i-123"
	nodeID := uuid.New()
	exec := &fakeExecutor{
		fetchResult: awssvc.CommandResult{
			Status: "Success",
			Stdout: `{"apps":{"http":{"servers":{"srv0":{"routes":[{"@id":"route-main","handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"10.0.0.1:8080"},{"dial":"10.0.0.2:8080"}]}],"match":[{"host":["main.example.com"]}]},{"@id":"route-no-upstreams","handle":[{"handler":"static_response"}]}]}}}}}`,
		},
	}
	svc := NewService(
		&fakeNodeLoader{node: &models.CaddyNode{ID: nodeID, Region: "eu-west-1", InstanceID: &instanceID}},
		&fakeDiscoveryLoader{},
		&fakeSnapshotWriter{},
		exec,
		WithCacheTTL(time.Minute),
	)

	ids, err := svc.ListConfigIDs(context.Background(), nodeID)
	if err != nil {
		t.Fatalf("ListConfigIDs: unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("ListConfigIDs: got %d items, want 2", len(ids))
	}
	if ids[0].ID != "route-main" || !ids[0].HasUpstreams || ids[0].UpstreamCount != 2 || ids[0].HostCount != 1 {
		t.Fatalf("route-main summary mismatch: %+v", ids[0])
	}
	if ids[1].ID != "route-no-upstreams" || ids[1].HasUpstreams || ids[1].UpstreamCount != 0 || ids[1].HostCount != 0 {
		t.Fatalf("route-no-upstreams summary mismatch: %+v", ids[1])
	}

	fragment, err := svc.GetConfigByID(context.Background(), nodeID, "route-main")
	if err != nil {
		t.Fatalf("GetConfigByID: unexpected error: %v", err)
	}
	if len(fragment) == 0 {
		t.Fatal("GetConfigByID: expected non-empty JSON fragment")
	}

	upstreams, err := svc.GetUpstreamsByID(context.Background(), nodeID, "route-main")
	if err != nil {
		t.Fatalf("GetUpstreamsByID: unexpected error: %v", err)
	}
	if len(upstreams) != 2 {
		t.Fatalf("GetUpstreamsByID: got %d upstreams, want 2", len(upstreams))
	}

	_, err = svc.GetConfigByID(context.Background(), nodeID, "missing-id")
	if !errors.Is(err, ErrConfigIDNotFound) {
		t.Fatalf("GetConfigByID missing: got %v, want ErrConfigIDNotFound", err)
	}
}

func TestService_GetLiveConfig_UsesCacheUntilInvalidated(t *testing.T) {
	instanceID := "i-abc"
	nodeID := uuid.New()
	exec := &fakeExecutor{
		fetchResult: awssvc.CommandResult{
			Status: "Success",
			Stdout: `{"apps":{"http":{"servers":{"srv0":{"routes":[{"@id":"first"}]}}}}}`,
		},
	}
	svc := NewService(
		&fakeNodeLoader{node: &models.CaddyNode{ID: nodeID, Region: "eu-west-1", InstanceID: &instanceID}},
		&fakeDiscoveryLoader{},
		&fakeSnapshotWriter{},
		exec,
		WithCacheTTL(time.Minute),
	)

	first, err := svc.GetLiveConfig(context.Background(), nodeID)
	if err != nil {
		t.Fatalf("GetLiveConfig first: unexpected error: %v", err)
	}
	second, err := svc.GetLiveConfig(context.Background(), nodeID)
	if err != nil {
		t.Fatalf("GetLiveConfig second: unexpected error: %v", err)
	}
	if exec.fetchCalls != 1 {
		t.Fatalf("FetchConfig calls=%d, want 1 cache hit", exec.fetchCalls)
	}
	if string(first) != string(second) {
		t.Fatalf("cache mismatch first=%s second=%s", string(first), string(second))
	}

	if err := svc.Reload(context.Background(), nodeID); err != nil {
		t.Fatalf("Reload: unexpected error: %v", err)
	}

	exec.fetchResult.Stdout = `{"apps":{"http":{"servers":{"srv0":{"routes":[{"@id":"second"}]}}}}}`
	third, err := svc.GetLiveConfig(context.Background(), nodeID)
	if err != nil {
		t.Fatalf("GetLiveConfig third: unexpected error: %v", err)
	}
	if exec.fetchCalls != 2 {
		t.Fatalf("FetchConfig calls=%d, want 2 after invalidation", exec.fetchCalls)
	}
	if string(third) == string(first) {
		t.Fatalf("expected refreshed config after reload invalidation")
	}
}

func TestService_GetHostsByID(t *testing.T) {
	instanceID := "i-hosts"
	nodeID := uuid.New()
	exec := &fakeExecutor{
		fetchResult: awssvc.CommandResult{
			Status: "Success",
			Stdout: `{"apps":{"http":{"servers":{"srv0":{"routes":[{"@id":"route-hosts","handle":[{"handler":"reverse_proxy","upstreams":[{"dial":"172.31.10.245:5555"}]}],"match":[{"host":["flower.gruppogaspari.it","alt.gruppogaspari.it","flower.gruppogaspari.it"]}],"terminal":true}]}}}}}`,
		},
	}
	svc := NewService(
		&fakeNodeLoader{node: &models.CaddyNode{ID: nodeID, Region: "eu-west-1", InstanceID: &instanceID}},
		&fakeDiscoveryLoader{},
		&fakeSnapshotWriter{},
		exec,
		WithCacheTTL(time.Minute),
	)

	hosts, err := svc.GetHostsByID(context.Background(), nodeID, "route-hosts")
	if err != nil {
		t.Fatalf("GetHostsByID: unexpected error: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("GetHostsByID: got %d hosts, want 2 (%v)", len(hosts), hosts)
	}
	if hosts[0] != "alt.gruppogaspari.it" || hosts[1] != "flower.gruppogaspari.it" {
		t.Fatalf("GetHostsByID: unexpected hosts order/content: %v", hosts)
	}
}
