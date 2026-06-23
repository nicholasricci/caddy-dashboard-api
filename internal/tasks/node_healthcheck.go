package tasks

import (
	"context"
	"encoding/json"
	"time"

	caddysvc "github.com/nicholasricci/caddy-dashboard/internal/caddy"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
)

type nodeHealthcheckTask struct {
	nodeRepo *repository.NodeRepository
	caddySvc *caddysvc.Service
}

func NewNodeHealthcheckTask(nodeRepo *repository.NodeRepository, caddySvc *caddysvc.Service) TaskRunner {
	return &nodeHealthcheckTask{nodeRepo: nodeRepo, caddySvc: caddySvc}
}

var _ TaskRunner = (*nodeHealthcheckTask)(nil)

func (t *nodeHealthcheckTask) Name() string {
	return "node_healthcheck"
}

type nodeHealthResult struct {
	NodeID string `json:"node_id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

func (t *nodeHealthcheckTask) Run(ctx context.Context, _ json.RawMessage) (*TaskResult, error) {
	start := time.Now()
	nodes, err := t.nodeRepo.List(ctx)
	if err != nil {
		return FailedResult(err), nil
	}

	results := make([]nodeHealthResult, 0, len(nodes))
	for _, node := range nodes {
		if node.EffectiveTransport() == models.TransportInventoryOnly {
			continue
		}

		t.caddySvc.PurgeNodeState(node.ID)
		_, err := t.caddySvc.GetLiveConfig(ctx, node.ID)
		r := nodeHealthResult{
			NodeID: node.ID.String(),
			Name:   node.Name,
		}
		if err != nil {
			r.Status = "unhealthy"
			r.Error = err.Error()
		} else {
			r.Status = "healthy"
		}
		results = append(results, r)
	}

	healthyCount := 0
	unhealthyCount := 0
	for _, r := range results {
		if r.Status == "healthy" {
			healthyCount++
		} else {
			unhealthyCount++
		}
	}

	return SuccessResult(map[string]any{
		"duration_ms":  durationMs(start),
		"total_nodes":  len(results),
		"healthy":      healthyCount,
		"unhealthy":    unhealthyCount,
		"node_results": results,
	}), nil
}
