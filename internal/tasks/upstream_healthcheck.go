package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
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

type upstreamHealthcheckConfig struct {
	ConfigIDs []string `json:"config_ids"`
}

type discoveryUpstreamResult struct {
	DiscoveryConfigID string   `json:"discovery_config_id"`
	DiscoveryName     string   `json:"discovery_name"`
	LeaderNodeID      string   `json:"leader_node_id"`
	ConfigIDsChecked  []string `json:"config_ids_checked,omitempty"`
	DialsChecked      int      `json:"dials_checked"`
	UnhealthyDials    int      `json:"unhealthy_dials"`
	Changed           bool     `json:"changed"`
	Pruned            []string `json:"pruned,omitempty"`
	Error             string   `json:"error,omitempty"`
}

func (t *upstreamHealthcheckTask) Run(ctx context.Context, raw json.RawMessage) (*TaskResult, error) {
	start := time.Now()

	var cfg upstreamHealthcheckConfig
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return FailedResult(fmt.Errorf("invalid upstream_healthcheck config: %w", err)), nil
		}
	}

	configIDFilter := make(map[string]bool, len(cfg.ConfigIDs))
	for _, id := range cfg.ConfigIDs {
		configIDFilter[strings.TrimSpace(id)] = true
	}

	discoveries, err := t.discoveryRepo.List(ctx)
	if err != nil {
		return FailedResult(err), nil
	}

	results := make([]discoveryUpstreamResult, 0, len(discoveries))
	for _, disc := range discoveries {
		r := t.checkDiscovery(ctx, disc, configIDFilter)
		results = append(results, r)
	}

	return SuccessResult(map[string]any{
		"duration_ms":       durationMs(start),
		"discoveries":       len(results),
		"discovery_results": results,
	}), nil
}

func (t *upstreamHealthcheckTask) checkDiscovery(ctx context.Context, disc models.DiscoveryConfig, configIDFilter map[string]bool) discoveryUpstreamResult {
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

	dialsByConfig := make(map[string][]string)
	checkedIDs := make([]string, 0)

	for _, idInfo := range ids {
		if !idInfo.HasUpstreams {
			continue
		}
		if len(configIDFilter) > 0 && !configIDFilter[idInfo.ID] {
			continue
		}

		upstreams, err := t.caddySvc.GetUpstreamsByID(ctx, *leaderID, idInfo.ID)
		if err != nil {
			continue
		}

		dials := make([]string, 0, len(upstreams))
		for _, raw := range upstreams {
			var entry map[string]string
			if err := json.Unmarshal(raw, &entry); err != nil {
				continue
			}
			if dial := strings.TrimSpace(entry["dial"]); dial != "" {
				dials = append(dials, dial)
			}
		}

		if len(dials) > 0 {
			dialsByConfig[idInfo.ID] = dials
			checkedIDs = append(checkedIDs, idInfo.ID)
		}
	}
	r.ConfigIDsChecked = checkedIDs

	if len(dialsByConfig) == 0 {
		return r
	}

	allDials := make([]string, 0)
	dialToConfigIDs := make(map[string][]string)
	for configID, dials := range dialsByConfig {
		for _, dial := range dials {
			dialToConfigIDs[dial] = append(dialToConfigIDs[dial], configID)
			allDials = append(allDials, dial)
		}
	}
	allDials = uniqueStrings(allDials)
	r.DialsChecked = len(allDials)

	remoteScript := buildTCPCheckScript(allDials)
	execRes, err := t.caddySvc.RunRemoteCommand(ctx, *leaderID, remoteScript)
	if err != nil {
		r.Error = fmt.Sprintf("remote command failed: %v", err)
		return r
	}
	if execRes.Status != caddysvc.ExecStatusSuccess {
		r.Error = fmt.Sprintf("remote command: status=%s stderr=%s", execRes.Status, execRes.Stderr)
		return r
	}

	unhealthyDials := parseHealthCheckOutput(execRes.Stdout)
	r.UnhealthyDials = len(unhealthyDials)

	if len(unhealthyDials) == 0 {
		return r
	}

	targets := make([]caddysvc.UpstreamMutationTarget, 0, len(unhealthyDials))
	for dial := range unhealthyDials {
		for _, configID := range dialToConfigIDs[dial] {
			targets = append(targets, caddysvc.UpstreamMutationTarget{
				ConfigID:   configID,
				RemoveDial: dial,
			})
		}
	}

	if len(targets) == 0 {
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
	r.Pruned = uniqueSortedStrings(mutateResp.Diff.Pruned)

	if !r.Changed {
		return r
	}

	if err := t.auditSvc.Record(ctx, "scheduler", "mutate_upstreams", "node", leaderID.String(), map[string]any{
		"discovery_config_id": disc.ID.String(),
		"pruned":              r.Pruned,
		"source":              "scheduler/upstream_healthcheck",
	}); err != nil {
		_ = err
	}

	_, err = t.caddySvc.PropagateToDiscoveryPeers(ctx, *leaderID, "scheduler/upstream_healthcheck")
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

	return r
}

func buildTCPCheckScript(dials []string) string {
	var b strings.Builder
	for _, dial := range dials {
		host, port, err := net.SplitHostPort(dial)
		if err != nil {
			continue
		}
		bashHost := host
		if strings.Contains(host, ":") {
			bashHost = "[" + host + "]"
		}
		fmt.Fprintf(&b, `timeout 2 bash -c 'echo > /dev/tcp/%s/%s' 2>/dev/null && echo "reachable|%s" || echo "unreachable|%s"`+"\n",
			bashHost, port, dial, dial)
	}
	return b.String()
}

func parseHealthCheckOutput(stdout string) map[string]bool {
	unhealthy := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.Index(line, "|")
		if idx < 0 {
			continue
		}
		status := strings.TrimSpace(line[:idx])
		dial := strings.TrimSpace(line[idx+1:])
		if dial == "" {
			continue
		}
		if status == "unreachable" {
			unhealthy[dial] = true
		}
	}
	return unhealthy
}

func uniqueStrings(vals []string) []string {
	seen := make(map[string]struct{}, len(vals))
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

func uniqueSortedStrings(vals []string) []string {
	out := uniqueStrings(vals)
	sort.Strings(out)
	return out
}
