package models

// CaddyConfigIDInfo describes one @id discovered in live Caddy config.
type CaddyConfigIDInfo struct {
	ID            string            `json:"id"`
	HasUpstreams  bool              `json:"has_upstreams"`
	UpstreamCount int               `json:"upstream_count"`
	HostCount     int               `json:"host_count"`
	Upstreams     []any             `json:"upstreams,omitempty"`
}

// CaddyConfigIDsResponse wraps discovered @id entries.
type CaddyConfigIDsResponse struct {
	Items []CaddyConfigIDInfo `json:"items"`
}

// CaddyConfigUpstreamsResponse wraps upstreams for one @id.
type CaddyConfigUpstreamsResponse struct {
	ID            string            `json:"id"`
	HasUpstreams  bool              `json:"has_upstreams"`
	UpstreamCount int               `json:"upstream_count"`
	Upstreams     []any             `json:"upstreams"`
}

// CaddyConfigHostsResponse wraps extracted hosts for one @id.
type CaddyConfigHostsResponse struct {
	ID        string   `json:"id"`
	HostCount int      `json:"host_count"`
	Hosts     []string `json:"hosts"`
}
