package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
)

// Runner discovers Azure VMs by resource tag in a resource group.
type Runner struct{}

// NewRunner returns an Azure discovery runner (uses DefaultAzureCredential).
func NewRunner() *Runner {
	return &Runner{}
}

type azureParams struct {
	SubscriptionID string `json:"subscription_id"`
	ResourceGroup  string `json:"resource_group"`
	TagName        string `json:"tag_name"`
	TagValue       string `json:"tag_value"`
	NodeTransport  string `json:"node_transport"` // optional: "inventory_only" (default) or "azure_run_command"
}

// Discover implements services.CloudDiscoverer for method azure_tags.
func (r *Runner) Discover(ctx context.Context, cfg *models.DiscoveryConfig) ([]models.CaddyNode, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil discovery config")
	}
	var p azureParams
	if err := json.Unmarshal(cfg.Parameters, &p); err != nil {
		return nil, fmt.Errorf("azure_tags parameters: %w", err)
	}
	if strings.TrimSpace(p.SubscriptionID) == "" || strings.TrimSpace(p.ResourceGroup) == "" ||
		strings.TrimSpace(p.TagName) == "" || strings.TrimSpace(p.TagValue) == "" {
		return nil, fmt.Errorf("azure_tags requires subscription_id, resource_group, tag_name, tag_value in parameters JSON")
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("azure credential: %w", err)
	}
	client, err := armcompute.NewVirtualMachinesClient(p.SubscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("compute client: %w", err)
	}

	pager := client.NewListPager(p.ResourceGroup, nil)
	var out []models.CaddyNode
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, vm := range page.Value {
			if vm == nil || vm.Name == nil {
				continue
			}
			if !tagMatches(vm.Tags, p.TagName, p.TagValue) {
				continue
			}
			name := *vm.Name
			id := fmt.Sprintf("azure:%s:%s:%s", p.SubscriptionID, p.ResourceGroup, name)
			status := "unknown"
			if vm.Properties != nil && vm.Properties.ProvisioningState != nil {
				status = *vm.Properties.ProvisioningState
			}
			tr := models.TransportInventoryOnly
			var tc json.RawMessage
			if strings.EqualFold(strings.TrimSpace(p.NodeTransport), models.TransportAzureRunCommand) {
				tr = models.TransportAzureRunCommand
				b, err := json.Marshal(map[string]string{
					"subscription_id": p.SubscriptionID,
					"resource_group":  p.ResourceGroup,
					"vm_name":         name,
				})
				if err != nil {
					return nil, fmt.Errorf("transport_config: %w", err)
				}
				tc = append(json.RawMessage(nil), b...)
			}
			node := models.CaddyNode{
				Name:       name,
				InstanceID: models.StringPtr(id),
				Region:     models.StringPtr(p.ResourceGroup),
				Transport:  tr,
				SSMEnabled: false,
				Status:     status,
			}
			if len(tc) > 0 {
				node.TransportConfig = tc
			}
			out = append(out, node)
		}
	}
	return out, nil
}

func tagMatches(tags map[string]*string, wantKey, wantVal string) bool {
	wantKey = strings.TrimSpace(strings.ToLower(wantKey))
	wantVal = strings.TrimSpace(wantVal)
	if wantKey == "" {
		return false
	}
	for k, v := range tags {
		if strings.EqualFold(strings.TrimSpace(k), wantKey) && v != nil && strings.TrimSpace(*v) == wantVal {
			return true
		}
	}
	return false
}
