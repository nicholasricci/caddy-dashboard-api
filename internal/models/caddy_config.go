package models

// CaddyConfigIDInfo describes one @id discovered in live Caddy config.
type CaddyConfigIDInfo struct {
	ID            string `json:"id"`
	HasUpstreams  bool   `json:"has_upstreams"`
	UpstreamCount int    `json:"upstream_count"`
	HostCount     int    `json:"host_count"`
	Upstreams     []any  `json:"upstreams,omitempty"`
}

// CaddyConfigIDsResponse wraps discovered @id entries.
type CaddyConfigIDsResponse struct {
	Items []CaddyConfigIDInfo `json:"items"`
}

// CaddyConfigUpstreamsResponse wraps upstreams for one @id.
type CaddyConfigUpstreamsResponse struct {
	ID            string `json:"id"`
	HasUpstreams  bool   `json:"has_upstreams"`
	UpstreamCount int    `json:"upstream_count"`
	Upstreams     []any  `json:"upstreams"`
}

// CaddyConfigHostsResponse wraps extracted hosts for one @id.
type CaddyConfigHostsResponse struct {
	ID        string   `json:"id"`
	HostCount int      `json:"host_count"`
	Hosts     []string `json:"hosts"`
}

type DomainMutationResult struct {
	ConfigID string   `json:"config_id"`
	Hosts    []string `json:"hosts"`
	Changed  bool     `json:"changed"`
	Added    []string `json:"added,omitempty"`
	Removed  []string `json:"removed,omitempty"`
}

type DomainMutationDiff struct {
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

type MutateDomainsResponse struct {
	Results []DomainMutationResult `json:"results"`
	Changed bool                   `json:"changed"`
	DryRun  bool                   `json:"dry_run"`
	Diff    DomainMutationDiff     `json:"diff"`
	Preview map[string]any         `json:"preview,omitempty"`
}

type UpstreamMutationResult struct {
	ConfigID  string   `json:"config_id"`
	Upstreams []string `json:"upstreams"`
	Pruned    []string `json:"pruned,omitempty"`
	Changed   bool     `json:"changed"`
	Added     []string `json:"added,omitempty"`
	Removed   []string `json:"removed,omitempty"`
}

type UpstreamMutationDiff struct {
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
	Pruned  []string `json:"pruned,omitempty"`
}

type MutateUpstreamsResponse struct {
	Results []UpstreamMutationResult `json:"results"`
	Changed bool                     `json:"changed"`
	DryRun  bool                     `json:"dry_run"`
	Diff    UpstreamMutationDiff     `json:"diff"`
	Preview map[string]any           `json:"preview,omitempty"`
}

type PropagateConfigResponse struct {
	SourceNodeID string   `json:"source_node_id"`
	AppliedTo    []string `json:"applied_to"`
	Skipped      []string `json:"skipped"`
}
