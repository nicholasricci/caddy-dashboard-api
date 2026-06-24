package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
)

type discoveryRunTask struct {
	discoverySvc interface {
		Run(ctx context.Context, id uuid.UUID) (int, error)
	}
	discoveryRepo *repository.DiscoveryRepository
}

func NewDiscoveryRunTask(discoverySvc interface {
	Run(ctx context.Context, id uuid.UUID) (int, error)
}, discoveryRepo *repository.DiscoveryRepository) TaskRunner {
	return &discoveryRunTask{discoverySvc: discoverySvc, discoveryRepo: discoveryRepo}
}

var _ TaskRunner = (*discoveryRunTask)(nil)

func (t *discoveryRunTask) Name() string {
	return models.ScheduledTaskTypeDiscoveryRun
}

type discoveryRunConfig struct {
	DiscoveryConfigID string `json:"discovery_config_id"`
}

func (t *discoveryRunTask) Run(ctx context.Context, rawConfig json.RawMessage) (*TaskResult, error) {
	var cfg discoveryRunConfig
	if err := json.Unmarshal(rawConfig, &cfg); err != nil {
		return FailedString(fmt.Sprintf("invalid config: %v", err)), nil
	}
	id, err := uuid.Parse(cfg.DiscoveryConfigID)
	if err != nil {
		return FailedString(fmt.Sprintf("invalid discovery_config_id: %v", err)), nil
	}
	start := time.Now()
	taskCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	nodesFound, err := t.discoverySvc.Run(taskCtx, id)
	if err != nil {
		return FailedResult(fmt.Errorf("discovery run failed: %w", err)), nil
	}
	return SuccessResult(map[string]any{
		"discovery_config_id": id.String(),
		"nodes_found":         nodesFound,
		"duration_ms":         durationMs(start),
	}), nil
}
