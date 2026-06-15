package models

import "strings"

// Node transport / remote execution backends.
const (
	TransportAWSSSM          = "aws_ssm"
	TransportSSH             = "ssh"
	TransportHTTPAdmin       = "http_admin"
	TransportInventoryOnly   = "inventory_only"
	TransportGCPOsConfig     = "gcp_osconfig"
	TransportAzureRunCommand = "azure_run_command"
)

// ValidNodeTransports lists accepted API values for CaddyNode.Transport.
var ValidNodeTransports = map[string]struct{}{
	TransportAWSSSM:          {},
	TransportSSH:             {},
	TransportHTTPAdmin:       {},
	TransportInventoryOnly:   {},
	TransportGCPOsConfig:     {},
	TransportAzureRunCommand: {},
}

// EffectiveTransport returns the transport to use for execution, defaulting to AWS SSM for legacy rows.
func (n *CaddyNode) EffectiveTransport() string {
	if n == nil {
		return TransportAWSSSM
	}
	t := strings.TrimSpace(n.Transport)
	if t == "" {
		return TransportAWSSSM
	}
	return t
}

// RegionString returns the AWS region or empty when unset (non-AWS nodes).
func (n *CaddyNode) RegionString() string {
	if n == nil || n.Region == nil {
		return ""
	}
	return strings.TrimSpace(*n.Region)
}

// StringPtr returns a pointer to a non-empty trimmed string, or nil if empty.
func StringPtr(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
