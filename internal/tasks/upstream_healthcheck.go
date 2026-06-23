package tasks

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	caddysvc "github.com/nicholasricci/caddy-dashboard/internal/caddy"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
)

type upstreamHealthcheckTask struct {
	caddySvc      *caddysvc.Service
	nodeRepo      *repository.NodeRepository
	discoveryRepo *repository.DiscoveryRepository
	auditSvc      *services.AuditService
}

func NewUpstreamHealthcheckTask(caddySvc *caddysvc.Service, nodeRepo *repository.NodeRepository, discoveryRepo *repository.DiscoveryRepository, auditSvc *services.AuditService) TaskRunner {
	return &upstreamHealthcheckTask{
		caddySvc:      caddySvc,
		nodeRepo:      nodeRepo,
		discoveryRepo: discoveryRepo,
		auditSvc:      auditSvc,
	}
}

var _ TaskRunner = (*upstreamHealthcheckTask)(nil)

func (t *upstreamHealthcheckTask) Name() string {
	return "upstream_healthcheck"
}

type discoveryUpstreamResult struct {
	DiscoveryConfigID string   `json:"discovery_config_id"`
	DiscoveryName     string   `json:"discovery_name"`
	LeaderNodeID      string   `json:"leader_node_id"`
	Changed           bool     `json:"changed"`
	Pruned            []string `json:"pruned,omitempty"`
	Error             string   `json:"error,omitempty"`
}

func (t *upstreamHealthcheckTask) Run(ctx context.Context, _ json.RawMessage) (*TaskResult, error) {
	start := time.Now()
	discoveries, err := t.discoveryRepo.List(ctx)
	if err != nil {
		return FailedResult(err), nil
	}

	results := make([]discoveryUpstreamResult, 0, len(discoveries))
	for _, disc := range discoveries {
		r := t.checkDiscovery(ctx, disc)
		results = append(results, r)
	}

	return SuccessResult(map[string]any{
		"duration_ms":       durationMs(start),
		"discoveries":       len(results),
		"discovery_results": results,
	}), nil
}

func (t *upstreamHealthcheckTask) checkDiscovery(ctx context.Context, disc models.DiscoveryConfig) discoveryUpstreamResult {
	r := discoveryUpstreamResult{
		DiscoveryConfigID: disc.ID.String(),
		DiscoveryName:     disc.Name,
	}

	nodes, err := t.nodeRepo.ListByDiscoveryConfigID(ctx, disc.ID)
	if err != nil {
		r.Error = err.Error()
		return r
	}

	var leaderID *uuid.UUID
	for _, node := range nodes {
		if node.EffectiveTransport() != models.TransportInventoryOnly {
			leaderID = &node.ID
			break
		}
	}
	if leaderID == nil {
		r.Error = "no reachable node"
		return r
	}
	r.LeaderNodeID = leaderID.String()

	t.caddySvc.PurgeNodeState(*leaderID)
	ids, err := t.caddySvc.ListConfigIDs(ctx, *leaderID)
	if err != nil {
		r.Error = err.Error()
		return r
	}

	targets := make([]caddysvc.UpstreamMutationTarget, 0)
	for _, idInfo := range ids {
		if idInfo.HasUpstreams {
			targets = append(targets, caddysvc.UpstreamMutationTarget{
				ConfigID:       idInfo.ID,
				PruneUnhealthy: true,
				ProbeTimeout:   2 * time.Second,
			})
		}
	}

	if len(targets) == 0 {
		r.Changed = false
		return r
	}

	mutateResp, err := t.caddySvc.MutateUpstreams(ctx, *leaderID, caddysvc.MutateUpstreamsRequest{
		Targets: targets,
		DryRun:  false,
	}, "scheduler/upstream_healthcheck")
	if err != nil {
		r.Error = err.Error()
		return r
	}

	r.Changed = mutateResp.Changed
	r.Pruned = mutateResp.Diff.Pruned

	if r.Changed {
		if err := t.auditSvc.Record(ctx, "scheduler", "mutate_upstreams", "node", leaderID.String(), map[string]any{
			"discovery_config_id": disc.ID.String(),
			"pruned":              mutateResp.Diff.Pruned,
			"source":              "scheduler/upstream_healthcheck",
		}); err != nil {
			_ = err // non-fatal
		}

		_, err := t.caddySvc.PropagateToDiscoveryPeers(ctx, *leaderID, "scheduler/upstream_healthcheck")
		if err != nil {
			r.Error = "mutate ok, but propagate failed: " + err.Error()
			return r
		}

		if err := t.auditSvc.Record(ctx, "scheduler", "propagate", "discovery", disc.ID.String(), map[string]any{
			"source_node_id": leaderID.String(),
			"source":         "scheduler/upstream_healthcheck",
		}); err != nil {
			_ = err
		}
	}

	return r
}
