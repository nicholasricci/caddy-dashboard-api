package caddy

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"net/url"
	"strings"

	"github.com/nicholasricci/caddy-dashboard/internal/models"
)

type rawTransportConfig struct {
	BaseURL          string `json:"base_url"`
	BearerTokenRef   string `json:"bearer_token_ref"`
	TLSSkipVerify    bool   `json:"tls_insecure_skip_verify"`
	ClientCertRef    string `json:"client_cert_ref"`
	ClientKeyRef     string `json:"client_key_ref"`
	Host             string `json:"host"`
	User             string `json:"user"`
	Port             int    `json:"port"`
	PrivateKeyRef    string `json:"private_key_ref"`
	KnownHostsRef    string `json:"known_hosts_ref"`
	KnownHostsPolicy string `json:"known_hosts_policy"`
}

// BuildExecTarget validates the node and builds an ExecTarget for the dispatcher.
func BuildExecTarget(node *models.CaddyNode) (ExecTarget, error) {
	if node == nil {
		return ExecTarget{}, fmt.Errorf("nil node")
	}
	tr := node.EffectiveTransport()
	switch tr {
	case models.TransportInventoryOnly:
		return ExecTarget{Node: node, Transport: tr}, nil

	case models.TransportAWSSSM:
		if node.InstanceID == nil || strings.TrimSpace(*node.InstanceID) == "" {
			return ExecTarget{}, ErrNodeNoInstanceID
		}
		if node.RegionString() == "" {
			return ExecTarget{}, fmt.Errorf("%w: aws_ssm requires region", ErrTransportNotConfigured)
		}
		return ExecTarget{Node: node, Transport: tr}, nil

	case models.TransportHTTPAdmin:
		raw := rawFromTransportConfig(node)
		var rc rawTransportConfig
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &rc); err != nil {
				return ExecTarget{}, fmt.Errorf("%w: invalid transport_config json: %v", ErrTransportNotConfigured, err)
			}
		}
		u := strings.TrimSpace(rc.BaseURL)
		if u == "" {
			return ExecTarget{}, fmt.Errorf("%w: http_admin requires transport_config.base_url", ErrTransportNotConfigured)
		}
		parsed, err := url.Parse(u)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return ExecTarget{}, fmt.Errorf("%w: invalid base_url", ErrTransportNotConfigured)
		}
		return ExecTarget{
			Node:      node,
			Transport: tr,
			HTTP: &HTTPAdminParams{
				BaseURL:        strings.TrimRight(u, "/"),
				BearerTokenRef: strings.TrimSpace(rc.BearerTokenRef),
				TLSSkipVerify:  rc.TLSSkipVerify,
				ClientCertRef:  strings.TrimSpace(rc.ClientCertRef),
				ClientKeyRef:   strings.TrimSpace(rc.ClientKeyRef),
			},
		}, nil

	case models.TransportSSH:
		raw := rawFromTransportConfig(node)
		var rc rawTransportConfig
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &rc); err != nil {
				return ExecTarget{}, fmt.Errorf("%w: invalid transport_config json: %v", ErrTransportNotConfigured, err)
			}
		}
		host := strings.TrimSpace(rc.Host)
		if host == "" && node.PrivateIP != nil {
			host = strings.TrimSpace(*node.PrivateIP)
		}
		if host == "" {
			return ExecTarget{}, fmt.Errorf("%w: ssh requires host or private_ip", ErrTransportNotConfigured)
		}
		if err := validateHostOrIP(host); err != nil {
			return ExecTarget{}, fmt.Errorf("%w: %v", ErrTransportNotConfigured, err)
		}
		user := strings.TrimSpace(rc.User)
		if user == "" {
			return ExecTarget{}, fmt.Errorf("%w: ssh requires user in transport_config", ErrTransportNotConfigured)
		}
		if strings.TrimSpace(rc.PrivateKeyRef) == "" {
			return ExecTarget{}, fmt.Errorf("%w: ssh requires private_key_ref in transport_config", ErrTransportNotConfigured)
		}
		port := rc.Port
		if port <= 0 {
			port = 22
		}
		policy := strings.TrimSpace(strings.ToLower(rc.KnownHostsPolicy))
		if policy == "" {
			policy = "secure"
		}
		if policy != "secure" && policy != "insecure" {
			return ExecTarget{}, fmt.Errorf("%w: known_hosts_policy must be secure or insecure", ErrTransportNotConfigured)
		}
		if policy == "secure" && strings.TrimSpace(rc.KnownHostsRef) == "" {
			return ExecTarget{}, fmt.Errorf("%w: ssh requires known_hosts_ref when known_hosts_policy is secure", ErrTransportNotConfigured)
		}
		return ExecTarget{
			Node:      node,
			Transport: tr,
			SSH: &SSHExecParams{
				Host:             host,
				User:             user,
				Port:             port,
				PrivateKeyRef:    strings.TrimSpace(rc.PrivateKeyRef),
				KnownHostsRef:    strings.TrimSpace(rc.KnownHostsRef),
				KnownHostsPolicy: policy,
			},
		}, nil

	default:
		return ExecTarget{}, fmt.Errorf("%w: %q", ErrUnknownTransport, tr)
	}
}

func rawFromTransportConfig(node *models.CaddyNode) []byte {
	if node == nil || len(node.TransportConfig) == 0 {
		return nil
	}
	return []byte(node.TransportConfig)
}

func validateHostOrIP(host string) error {
	if _, err := netip.ParseAddr(host); err == nil {
		return nil
	}
	// Allow hostnames (simple sanity check).
	if len(host) > 253 || strings.Contains(host, " ") {
		return fmt.Errorf("invalid host %q", host)
	}
	return nil
}
