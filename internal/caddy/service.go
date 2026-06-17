package caddy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
	"gorm.io/datatypes"
)

// ErrConfigIDNotFound is returned when a requested @id is not present in the live config.
var ErrConfigIDNotFound = errors.New("config @id not found")

// nodeLoader is satisfied by *repository.NodeRepository; narrowed for tests.
type nodeLoader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.CaddyNode, error)
	ListByDiscoveryConfigID(ctx context.Context, discoveryConfigID uuid.UUID) ([]models.CaddyNode, error)
}

type discoveryLoader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.DiscoveryConfig, error)
}

type snapshotWriter interface {
	Create(ctx context.Context, s *models.CaddySnapshot) error
}

type Service struct {
	nodes       nodeLoader
	discoveries discoveryLoader
	snapshots   snapshotWriter
	executor    RemoteExecutor
	cache       ConfigCacheStore
	cacheTTL    time.Duration
	locksMu     sync.Mutex
	locks       map[uuid.UUID]*sync.Mutex
}

type MutateDomainsRequest struct {
	Targets           []DomainMutationTarget
	TLSDNSChallenge   *TLSDNSChallenge
	UpdateTLSPolicies bool
	DryRun            bool
}

type MutateDomainsResponse struct {
	Results []DomainMutationResult     `json:"results"`
	Changed bool                       `json:"changed"`
	DryRun  bool                       `json:"dry_run"`
	Diff    DomainMutationDiff         `json:"diff"`
	Preview map[string]json.RawMessage `json:"preview,omitempty"`
}

type MutateUpstreamsRequest struct {
	Targets []UpstreamMutationTarget
	DryRun  bool
}

type MutateUpstreamsResponse struct {
	Results []UpstreamMutationResult   `json:"results"`
	Changed bool                       `json:"changed"`
	DryRun  bool                       `json:"dry_run"`
	Diff    UpstreamMutationDiff       `json:"diff"`
	Preview map[string]json.RawMessage `json:"preview,omitempty"`
}

type PropagateResponse struct {
	SourceNodeID uuid.UUID   `json:"source_node_id"`
	AppliedTo    []uuid.UUID `json:"applied_to"`
	Skipped      []uuid.UUID `json:"skipped"`
}

func NewService(nodes nodeLoader, discoveries discoveryLoader, snapshots snapshotWriter, executor RemoteExecutor, opts ...Option) *Service {
	s := &Service{
		nodes:       nodes,
		discoveries: discoveries,
		snapshots:   snapshots,
		executor:    executor,
		cache:       NewInMemoryConfigCacheStore(),
		cacheTTL:    2 * time.Minute,
		locks:       make(map[uuid.UUID]*sync.Mutex),
	}
	for _, opt := range opts {
		opt(s)
	}
	if s.cache == nil {
		s.cache = NewInMemoryConfigCacheStore()
	}
	if s.cacheTTL <= 0 {
		s.cacheTTL = 2 * time.Minute
	}
	return s
}

func (s *Service) SyncNodeConfig(ctx context.Context, nodeID uuid.UUID, requestedBy string) error {
	node, err := s.nodes.GetByID(ctx, nodeID)
	if err != nil {
		if repository.IsNotFound(err) {
			return models.ErrNodeNotFound
		}
		return err
	}
	if node.EffectiveTransport() == models.TransportInventoryOnly {
		return ErrTransportUnsupportedOp
	}
	target, err := BuildExecTarget(node)
	if err != nil {
		return err
	}
	res, err := s.executor.FetchConfig(ctx, target)
	if err != nil {
		return err
	}
	if res.Status != ExecStatusSuccess {
		return fmt.Errorf("remote fetch config: status=%s stderr=%s", res.Status, res.Stderr)
	}
	payload := []byte(res.Stdout)
	if err := s.storeSnapshot(ctx, node, payload, requestedBy); err != nil {
		return err
	}
	if _, err := s.cacheIndexedConfig(nodeID, payload, "ssm"); err != nil {
		s.cache.Invalidate(nodeID)
	}
	return nil
}

// GetLiveConfig returns the current Caddy JSON config from the node admin API (same SSM fetch as sync) without persisting a snapshot.
func (s *Service) GetLiveConfig(ctx context.Context, nodeID uuid.UUID) (json.RawMessage, error) {
	cfg, err := s.getIndexedConfig(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return cfg.Raw, nil
}

func (s *Service) ListConfigIDs(ctx context.Context, nodeID uuid.UUID) ([]models.CaddyConfigIDInfo, error) {
	cfg, err := s.getIndexedConfig(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	out := make([]models.CaddyConfigIDInfo, len(cfg.IDEntries))
	copy(out, cfg.IDEntries)
	return out, nil
}

func (s *Service) GetConfigByID(ctx context.Context, nodeID uuid.UUID, configID string) (json.RawMessage, error) {
	cfg, err := s.getIndexedConfig(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	raw, ok := cfg.ConfigByID[configID]
	if !ok {
		return nil, ErrConfigIDNotFound
	}
	return raw, nil
}

func (s *Service) GetUpstreamsByID(ctx context.Context, nodeID uuid.UUID, configID string) ([]json.RawMessage, error) {
	cfg, err := s.getIndexedConfig(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	raw, ok := cfg.ConfigByID[configID]
	if !ok || len(raw) == 0 {
		return nil, ErrConfigIDNotFound
	}
	upstreams := cfg.UpstreamMap[configID]
	out := make([]json.RawMessage, len(upstreams))
	copy(out, upstreams)
	return out, nil
}

func (s *Service) GetHostsByID(ctx context.Context, nodeID uuid.UUID, configID string) ([]string, error) {
	cfg, err := s.getIndexedConfig(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if _, ok := cfg.ConfigByID[configID]; !ok {
		return nil, ErrConfigIDNotFound
	}
	hosts := cfg.HostMap[configID]
	out := make([]string, len(hosts))
	copy(out, hosts)
	return out, nil
}

func extractHostsFromRaw(raw json.RawMessage) []string {
	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil
	}
	return uniqueSortedStrings(collectMatchHosts(node))
}

func uniqueSortedStrings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func buildHostMap(configByID map[string]json.RawMessage) map[string][]string {
	out := make(map[string][]string, len(configByID))
	for id, raw := range configByID {
		out[id] = extractHostsFromRaw(raw)
	}
	return out
}

func (s *Service) ApplyConfig(ctx context.Context, nodeID uuid.UUID, payload []byte, requestedBy string) error {
	node, err := s.nodes.GetByID(ctx, nodeID)
	if err != nil {
		if repository.IsNotFound(err) {
			return models.ErrNodeNotFound
		}
		return err
	}
	if node.EffectiveTransport() == models.TransportInventoryOnly {
		return ErrTransportUnsupportedOp
	}
	target, err := BuildExecTarget(node)
	if err != nil {
		return err
	}
	res, err := s.executor.ApplyConfig(ctx, target, payload)
	if err != nil {
		return err
	}
	if res.Status != ExecStatusSuccess {
		return fmt.Errorf("remote apply: status=%s stderr=%s", res.Status, res.Stderr)
	}
	if err := s.storeSnapshot(ctx, node, payload, requestedBy); err != nil {
		return err
	}
	if _, err := s.cacheIndexedConfig(nodeID, payload, "ssm"); err != nil {
		s.cache.Invalidate(nodeID)
	}
	return nil
}

func (s *Service) Reload(ctx context.Context, nodeID uuid.UUID) error {
	node, err := s.nodes.GetByID(ctx, nodeID)
	if err != nil {
		if repository.IsNotFound(err) {
			return models.ErrNodeNotFound
		}
		return err
	}
	if node.EffectiveTransport() == models.TransportInventoryOnly {
		return ErrTransportUnsupportedOp
	}
	target, err := BuildExecTarget(node)
	if err != nil {
		return err
	}
	res, err := s.executor.Reload(ctx, target)
	if err != nil {
		return err
	}
	if res.Status != ExecStatusSuccess {
		return fmt.Errorf("remote reload: status=%s stderr=%s", res.Status, res.Stderr)
	}
	s.cache.Invalidate(nodeID)
	return nil
}

func (s *Service) MutateDomains(ctx context.Context, nodeID uuid.UUID, req MutateDomainsRequest, requestedBy string) (*MutateDomainsResponse, error) {
	if len(req.Targets) == 0 {
		return nil, fmt.Errorf("%w: targets are required", ErrInvalidMutationPayload)
	}
	raw, err := s.GetLiveConfig(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	parsed, err := configToMap(raw)
	if err != nil {
		return nil, err
	}
	out := &MutateDomainsResponse{
		Results: make([]DomainMutationResult, 0, len(req.Targets)),
		DryRun:  req.DryRun,
	}
	changed := false
	for _, target := range req.Targets {
		res, err := mutateHostsForID(parsed, target)
		if err != nil {
			return nil, err
		}
		out.Results = append(out.Results, res)
		if res.Changed {
			changed = true
		}
		if req.UpdateTLSPolicies {
			added := make([]string, 0)
			removed := make([]string, 0)
			addSet := toStringSet(target.AddDomains)
			remSet := toStringSet(target.RemoveDomains)
			for d := range addSet {
				if strings.HasPrefix(d, "*.") {
					added = append(added, d)
				}
			}
			for d := range remSet {
				removed = append(removed, d)
			}
			ch1, err := applyTLSPolicyChanges(parsed, added, req.TLSDNSChallenge, false)
			if err != nil {
				return nil, err
			}
			ch2, err := applyTLSPolicyChanges(parsed, removed, nil, true)
			if err != nil {
				return nil, err
			}
			changed = changed || ch1 || ch2
		}
	}
	out.Changed = changed
	out.Diff = buildDomainDiff(out.Results)
	out.Preview = buildPreviewByDomainTargets(parsed, req.Targets)
	if !changed {
		return out, nil
	}
	if req.DryRun {
		return out, nil
	}
	payload, err := mapToConfig(parsed)
	if err != nil {
		return nil, err
	}
	if err := s.ApplyConfig(ctx, nodeID, payload, requestedBy); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) MutateUpstreams(ctx context.Context, nodeID uuid.UUID, req MutateUpstreamsRequest, requestedBy string) (*MutateUpstreamsResponse, error) {
	if len(req.Targets) == 0 {
		return nil, fmt.Errorf("%w: targets are required", ErrInvalidMutationPayload)
	}
	raw, err := s.GetLiveConfig(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	parsed, err := configToMap(raw)
	if err != nil {
		return nil, err
	}
	out := &MutateUpstreamsResponse{
		Results: make([]UpstreamMutationResult, 0, len(req.Targets)),
		DryRun:  req.DryRun,
	}
	for _, target := range req.Targets {
		res, err := mutateUpstreamsForID(parsed, target)
		if err != nil {
			return nil, err
		}
		out.Results = append(out.Results, res)
		out.Changed = out.Changed || res.Changed
	}
	out.Diff = buildUpstreamDiff(out.Results)
	out.Preview = buildPreviewByUpstreamTargets(parsed, req.Targets)
	if !out.Changed {
		return out, nil
	}
	if req.DryRun {
		return out, nil
	}
	payload, err := mapToConfig(parsed)
	if err != nil {
		return nil, err
	}
	if err := s.ApplyConfig(ctx, nodeID, payload, requestedBy); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) PropagateToDiscoveryPeers(ctx context.Context, sourceNodeID uuid.UUID, requestedBy string) (*PropagateResponse, error) {
	source, err := s.nodes.GetByID(ctx, sourceNodeID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, models.ErrNodeNotFound
		}
		return nil, err
	}
	if source.DiscoveryConfigID == nil || *source.DiscoveryConfigID == uuid.Nil {
		return &PropagateResponse{SourceNodeID: sourceNodeID}, nil
	}
	raw, err := s.GetLiveConfig(ctx, sourceNodeID)
	if err != nil {
		return nil, err
	}
	peers, err := s.nodes.ListByDiscoveryConfigID(ctx, *source.DiscoveryConfigID)
	if err != nil {
		return nil, err
	}
	resp := &PropagateResponse{
		SourceNodeID: sourceNodeID,
		AppliedTo:    make([]uuid.UUID, 0, len(peers)),
		Skipped:      make([]uuid.UUID, 0, len(peers)),
	}
	for _, peer := range peers {
		if peer.ID == sourceNodeID {
			continue
		}
		if peer.EffectiveTransport() == models.TransportInventoryOnly {
			resp.Skipped = append(resp.Skipped, peer.ID)
			continue
		}
		if err := s.ApplyConfig(ctx, peer.ID, raw, requestedBy); err != nil {
			resp.Skipped = append(resp.Skipped, peer.ID)
			continue
		}
		resp.AppliedTo = append(resp.AppliedTo, peer.ID)
	}
	return resp, nil
}

func (s *Service) getIndexedConfig(ctx context.Context, nodeID uuid.UUID) (indexedConfig, error) {
	if entry, ok := s.cache.Get(nodeID); ok && time.Now().UTC().Before(entry.ExpiresAt) {
		return entry.Config, nil
	}
	lock := s.nodeLock(nodeID)
	lock.Lock()
	defer lock.Unlock()
	if entry, ok := s.cache.Get(nodeID); ok && time.Now().UTC().Before(entry.ExpiresAt) {
		return entry.Config, nil
	}
	node, err := s.nodes.GetByID(ctx, nodeID)
	if err != nil {
		if repository.IsNotFound(err) {
			return indexedConfig{}, models.ErrNodeNotFound
		}
		return indexedConfig{}, err
	}
	if node.EffectiveTransport() == models.TransportInventoryOnly {
		return indexedConfig{}, ErrTransportUnsupportedOp
	}
	target, err := BuildExecTarget(node)
	if err != nil {
		return indexedConfig{}, err
	}
	res, err := s.executor.FetchConfig(ctx, target)
	if err != nil {
		return indexedConfig{}, err
	}
	if res.Status != ExecStatusSuccess {
		return indexedConfig{}, fmt.Errorf("remote fetch config: status=%s stderr=%s", res.Status, res.Stderr)
	}
	return s.cacheIndexedConfig(nodeID, []byte(res.Stdout), "remote")
}

func (s *Service) cacheIndexedConfig(nodeID uuid.UUID, raw []byte, source string) (indexedConfig, error) {
	_ = source
	indexed, err := buildIndexedConfig(raw)
	if err != nil {
		return indexedConfig{}, err
	}
	now := time.Now().UTC()
	s.cache.Set(nodeID, cacheEntry{
		FetchedAt: now,
		ExpiresAt: now.Add(s.cacheTTL),
		Config:    indexed,
	})
	return indexed, nil
}

func (s *Service) nodeLock(nodeID uuid.UUID) *sync.Mutex {
	s.locksMu.Lock()
	defer s.locksMu.Unlock()
	lock, ok := s.locks[nodeID]
	if !ok {
		lock = &sync.Mutex{}
		s.locks[nodeID] = lock
	}
	return lock
}

func buildIndexedConfig(raw []byte) (indexedConfig, error) {
	out := []byte(strings.TrimSpace(string(raw)))
	if len(out) == 0 {
		return indexedConfig{}, fmt.Errorf("empty config response from node")
	}
	var compact json.RawMessage
	if err := json.Unmarshal(out, &compact); err != nil {
		return indexedConfig{}, fmt.Errorf("config response is not valid json: %w", err)
	}
	var parsed any
	if err := json.Unmarshal(compact, &parsed); err != nil {
		return indexedConfig{}, fmt.Errorf("config response is not valid json: %w", err)
	}
	configByID := make(map[string]json.RawMessage)
	upstreamMap := make(map[string][]json.RawMessage)
	walkConfig(parsed, configByID, upstreamMap)
	hostMap := buildHostMap(configByID)
	ids := make([]string, 0, len(configByID))
	for id := range configByID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	entries := make([]models.CaddyConfigIDInfo, 0, len(ids))
	for _, id := range ids {
		upstreams := upstreamMap[id]
		cloned := make([]json.RawMessage, len(upstreams))
		copy(cloned, upstreams)
		entries = append(entries, models.CaddyConfigIDInfo{
			ID:            id,
			HasUpstreams:  len(upstreams) > 0,
			UpstreamCount: len(upstreams),
			HostCount:     len(hostMap[id]),
			Upstreams:     rawMessagesToAny(cloned),
		})
	}
	return indexedConfig{
		Raw:         compact,
		IDEntries:   entries,
		ConfigByID:  configByID,
		UpstreamMap: upstreamMap,
		HostMap:     hostMap,
	}, nil
}

func walkConfig(value any, configByID map[string]json.RawMessage, upstreamMap map[string][]json.RawMessage) {
	switch typed := value.(type) {
	case map[string]any:
		if id, ok := typed["@id"].(string); ok && strings.TrimSpace(id) != "" {
			if raw, err := json.Marshal(typed); err == nil {
				var compact json.RawMessage
				if err := json.Unmarshal(raw, &compact); err == nil {
					configByID[id] = compact
				}
			}
			upstreamMap[id] = collectUpstreams(typed)
		}
		for _, v := range typed {
			walkConfig(v, configByID, upstreamMap)
		}
	case []any:
		for _, v := range typed {
			walkConfig(v, configByID, upstreamMap)
		}
	}
}

func collectUpstreams(value any) []json.RawMessage {
	out := make([]json.RawMessage, 0)
	var walk func(any)
	walk = func(v any) {
		switch typed := v.(type) {
		case map[string]any:
			for k, item := range typed {
				if k == "upstreams" {
					if list, ok := item.([]any); ok {
						for _, u := range list {
							if b, err := json.Marshal(u); err == nil {
								var compact json.RawMessage
								if err := json.Unmarshal(b, &compact); err == nil {
									out = append(out, compact)
								}
							}
						}
					}
				}
				walk(item)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
	return out
}

func rawMessagesToAny(values []json.RawMessage) []any {
	out := make([]any, 0, len(values))
	for _, raw := range values {
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err == nil {
			out = append(out, decoded)
			continue
		}
		out = append(out, string(raw))
	}
	return out
}

func collectMatchHosts(value any) []string {
	out := make([]string, 0)
	var walk func(any)
	walk = func(v any) {
		switch typed := v.(type) {
		case map[string]any:
			for k, item := range typed {
				if k == "match" {
					if matches, ok := item.([]any); ok {
						for _, entry := range matches {
							entryMap, ok := entry.(map[string]any)
							if !ok {
								continue
							}
							rawHosts, ok := entryMap["host"].([]any)
							if !ok {
								continue
							}
							for _, h := range rawHosts {
								if hs, ok := h.(string); ok && strings.TrimSpace(hs) != "" {
									out = append(out, strings.TrimSpace(hs))
								}
							}
						}
					}
				}
				walk(item)
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
	return out
}

func (s *Service) storeSnapshot(ctx context.Context, node *models.CaddyNode, payload []byte, requestedBy string) error {
	var compact json.RawMessage
	if err := json.Unmarshal(payload, &compact); err != nil {
		return fmt.Errorf("payload is not valid json: %w", err)
	}

	scope := models.SnapshotScopeNode
	var discoveryConfigID *uuid.UUID
	if node.DiscoveryConfigID != nil && *node.DiscoveryConfigID != uuid.Nil {
		cfg, err := s.discoveries.GetByID(ctx, *node.DiscoveryConfigID)
		switch {
		case err == nil:
			scope = cfg.SnapshotScope
			if scope == models.SnapshotScopeGroup {
				discoveryConfigID = node.DiscoveryConfigID
			}
		case repository.IsNotFound(err):
			scope = models.SnapshotScopeNode
		default:
			return fmt.Errorf("load discovery config for node %s: %w", node.ID, err)
		}
	}

	snapshot := &models.CaddySnapshot{
		Config:            datatypes.JSON(compact),
		AppliedBy:         requestedBy,
		AppliedAt:         time.Now().UTC(),
		DiscoveryConfigID: discoveryConfigID,
	}
	if scope == models.SnapshotScopeNode {
		snapshot.NodeID = &node.ID
	}
	return s.snapshots.Create(ctx, snapshot)
}

type Option func(*Service)

func WithCache(store ConfigCacheStore) Option {
	return func(s *Service) {
		if store != nil {
			s.cache = store
		}
	}
}

func WithCacheTTL(ttl time.Duration) Option {
	return func(s *Service) {
		if ttl > 0 {
			s.cacheTTL = ttl
		}
	}
}

// PurgeNodeState drops per-node locks and cache entries (e.g. after node delete).
func (s *Service) PurgeNodeState(id uuid.UUID) {
	s.locksMu.Lock()
	delete(s.locks, id)
	s.locksMu.Unlock()
	s.cache.Invalidate(id)
}
