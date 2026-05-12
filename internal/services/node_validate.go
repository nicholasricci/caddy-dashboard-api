package services

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/nicholasricci/caddy-dashboard/internal/models"
)

// ValidateCaddyNode checks transport-specific required fields before persist.
func ValidateCaddyNode(n *models.CaddyNode) error {
	if n == nil {
		return fmt.Errorf("%w: nil node", ErrInvalidNodePayload)
	}
	tr := strings.TrimSpace(n.Transport)
	if tr == "" {
		tr = models.TransportAWSSSM
	}
	if _, ok := models.ValidNodeTransports[tr]; !ok {
		return fmt.Errorf("%w: invalid transport %q", ErrInvalidNodePayload, tr)
	}

	switch tr {
	case models.TransportAWSSSM:
		if n.RegionString() == "" {
			return fmt.Errorf("%w: aws_ssm requires region", ErrInvalidNodePayload)
		}
	case models.TransportHTTPAdmin:
		var cfg struct {
			BaseURL string `json:"base_url"`
		}
		if len(n.TransportConfig) == 0 {
			return fmt.Errorf("%w: http_admin requires transport_config with base_url", ErrInvalidNodePayload)
		}
		if err := json.Unmarshal(n.TransportConfig, &cfg); err != nil {
			return fmt.Errorf("%w: transport_config must be valid JSON", ErrInvalidNodePayload)
		}
		u := strings.TrimSpace(cfg.BaseURL)
		if u == "" {
			return fmt.Errorf("%w: http_admin requires transport_config.base_url", ErrInvalidNodePayload)
		}
		parsed, err := url.Parse(u)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("%w: invalid base_url", ErrInvalidNodePayload)
		}
	case models.TransportSSH:
		var cfg struct {
			Host             string `json:"host"`
			User             string `json:"user"`
			PrivateKeyRef    string `json:"private_key_ref"`
			KnownHostsRef    string `json:"known_hosts_ref"`
			KnownHostsPolicy string `json:"known_hosts_policy"`
		}
		if len(n.TransportConfig) == 0 {
			return fmt.Errorf("%w: ssh requires transport_config", ErrInvalidNodePayload)
		}
		if err := json.Unmarshal(n.TransportConfig, &cfg); err != nil {
			return fmt.Errorf("%w: transport_config must be valid JSON", ErrInvalidNodePayload)
		}
		host := strings.TrimSpace(cfg.Host)
		if host == "" && n.PrivateIP != nil {
			host = strings.TrimSpace(*n.PrivateIP)
		}
		if host == "" {
			return fmt.Errorf("%w: ssh requires host or private_ip", ErrInvalidNodePayload)
		}
		if strings.TrimSpace(cfg.User) == "" {
			return fmt.Errorf("%w: ssh requires transport_config.user", ErrInvalidNodePayload)
		}
		if strings.TrimSpace(cfg.PrivateKeyRef) == "" {
			return fmt.Errorf("%w: ssh requires transport_config.private_key_ref", ErrInvalidNodePayload)
		}
		policy := strings.TrimSpace(strings.ToLower(cfg.KnownHostsPolicy))
		if policy == "" {
			policy = "secure"
		}
		if policy == "secure" && strings.TrimSpace(cfg.KnownHostsRef) == "" {
			return fmt.Errorf("%w: ssh requires known_hosts_ref when known_hosts_policy is secure", ErrInvalidNodePayload)
		}
		if policy != "secure" && policy != "insecure" {
			return fmt.Errorf("%w: known_hosts_policy must be secure or insecure", ErrInvalidNodePayload)
		}
	case models.TransportInventoryOnly:
		// no extra requirements
	default:
		return fmt.Errorf("%w: unsupported transport %q", ErrInvalidNodePayload, tr)
	}
	return nil
}
