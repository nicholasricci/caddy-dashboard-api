package caddy

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
)

type indexedConfig struct {
	Raw         json.RawMessage
	IDEntries   []models.CaddyConfigIDInfo
	ConfigByID  map[string]json.RawMessage
	UpstreamMap map[string][]json.RawMessage
	HostMap     map[string][]string
}

type cacheEntry struct {
	FetchedAt time.Time
	ExpiresAt time.Time
	Config    indexedConfig
}

type ConfigCacheStore interface {
	Get(nodeID uuid.UUID) (cacheEntry, bool)
	Set(nodeID uuid.UUID, entry cacheEntry)
	Invalidate(nodeID uuid.UUID)
}

type InMemoryConfigCacheStore struct {
	mu      sync.RWMutex
	entries map[uuid.UUID]cacheEntry
}

func NewInMemoryConfigCacheStore() *InMemoryConfigCacheStore {
	return &InMemoryConfigCacheStore{
		entries: make(map[uuid.UUID]cacheEntry),
	}
}

func (s *InMemoryConfigCacheStore) Get(nodeID uuid.UUID) (cacheEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[nodeID]
	return entry, ok
}

func (s *InMemoryConfigCacheStore) Set(nodeID uuid.UUID, entry cacheEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[nodeID] = entry
}

func (s *InMemoryConfigCacheStore) Invalidate(nodeID uuid.UUID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, nodeID)
}
