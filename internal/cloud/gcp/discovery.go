package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

// Runner discovers GCE VMs by label in a single zone (parameters JSON).
type Runner struct{}

// NewRunner returns a GCP discovery runner (uses Application Default Credentials).
func NewRunner() *Runner {
	return &Runner{}
}

type gcpParams struct {
	ProjectID  string `json:"project_id"`
	Zone       string `json:"zone"`
	LabelKey   string `json:"label_key"`
	LabelValue string `json:"label_value"`
}

// Discover implements services.CloudDiscoverer for method gcp_labels.
func (r *Runner) Discover(ctx context.Context, cfg *models.DiscoveryConfig) ([]models.CaddyNode, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil discovery config")
	}
	var p gcpParams
	if err := json.Unmarshal(cfg.Parameters, &p); err != nil {
		return nil, fmt.Errorf("gcp_labels parameters: %w", err)
	}
	if strings.TrimSpace(p.ProjectID) == "" || strings.TrimSpace(p.Zone) == "" ||
		strings.TrimSpace(p.LabelKey) == "" || strings.TrimSpace(p.LabelValue) == "" {
		return nil, fmt.Errorf("gcp_labels requires project_id, zone, label_key, label_value in parameters JSON")
	}

	svc, err := compute.NewService(ctx, option.WithScopes(compute.ComputeReadonlyScope))
	if err != nil {
		return nil, fmt.Errorf("compute client: %w", err)
	}

	filter := fmt.Sprintf("labels.%s = %s", p.LabelKey, p.LabelValue)
	call := svc.Instances.List(p.ProjectID, p.Zone).Filter(filter)

	var out []models.CaddyNode
	err = call.Pages(ctx, func(page *compute.InstanceList) error {
		for _, inst := range page.Items {
			if inst == nil || inst.Name == "" {
				continue
			}
			var ipPtr *string
			if len(inst.NetworkInterfaces) > 0 && inst.NetworkInterfaces[0] != nil {
				ip := strings.TrimSpace(inst.NetworkInterfaces[0].NetworkIP)
				if ip != "" {
					ipPtr = &ip
				}
			}
			id := fmt.Sprintf("gcp:%s:%s:%s", p.ProjectID, p.Zone, inst.Name)
			status := strings.TrimSpace(inst.Status)
			if status == "" {
				status = "unknown"
			}
			out = append(out, models.CaddyNode{
				Name:       inst.Name,
				InstanceID: models.StringPtr(id),
				PrivateIP:  ipPtr,
				Region:     models.StringPtr(p.Zone),
				Transport:  models.TransportInventoryOnly,
				SSMEnabled: false,
				Status:     status,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
