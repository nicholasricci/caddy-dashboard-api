package caddy

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"
)

var (
	ErrInvalidMutationPayload = errors.New("invalid mutation payload")
	ErrConfigIDShapeMismatch  = errors.New("config id shape mismatch")
)

type DomainMutationTarget struct {
	ConfigID      string
	MatchIndexes  []int
	AddDomains    []string
	RemoveDomains []string
}

type TLSDNSChallenge struct {
	Provider string
	APIToken string
}

type DomainMutationResult struct {
	ConfigID string   `json:"config_id"`
	Hosts    []string `json:"hosts"`
	Changed  bool     `json:"changed"`
	Added    []string `json:"added,omitempty"`
	Removed  []string `json:"removed,omitempty"`
}

type UpstreamMutationTarget struct {
	ConfigID       string
	AddDial        string
	RemoveDial     string
	PruneUnhealthy bool
	ProbeTimeout   time.Duration
}

type UpstreamMutationResult struct {
	ConfigID  string   `json:"config_id"`
	Upstreams []string `json:"upstreams"`
	Pruned    []string `json:"pruned,omitempty"`
	Changed   bool     `json:"changed"`
	Added     []string `json:"added,omitempty"`
	Removed   []string `json:"removed,omitempty"`
}

type DomainMutationDiff struct {
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
}

type UpstreamMutationDiff struct {
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
	Pruned  []string `json:"pruned,omitempty"`
}

func mutateHostsForID(root map[string]any, t DomainMutationTarget) (DomainMutationResult, error) {
	cfg, ok := findConfigByID(root, strings.TrimSpace(t.ConfigID))
	if !ok {
		return DomainMutationResult{}, ErrConfigIDNotFound
	}
	match, ok := cfg["match"].([]any)
	if !ok || len(match) == 0 {
		return DomainMutationResult{}, fmt.Errorf("%w: %s has no match array", ErrConfigIDShapeMismatch, t.ConfigID)
	}
	indexes := t.MatchIndexes
	if len(indexes) == 0 {
		indexes = []int{0}
	}
	addSet := toStringSet(t.AddDomains)
	removeSet := toStringSet(t.RemoveDomains)
	changed := false
	finalHosts := make(map[string]struct{})
	for _, idx := range indexes {
		if idx < 0 || idx >= len(match) {
			return DomainMutationResult{}, fmt.Errorf("%w: invalid match index %d for %s", ErrInvalidMutationPayload, idx, t.ConfigID)
		}
		entry, ok := match[idx].(map[string]any)
		if !ok {
			return DomainMutationResult{}, fmt.Errorf("%w: match[%d] is not object", ErrConfigIDShapeMismatch, idx)
		}
		existing := anyToStringSlice(entry["host"])
		hostSet := toStringSet(existing)
		for h := range addSet {
			hostSet[h] = struct{}{}
		}
		for h := range removeSet {
			delete(hostSet, h)
		}
		next := sortedKeys(hostSet)
		if !stringSlicesEqual(existing, next) {
			entry["host"] = next
			changed = true
		}
		for _, h := range next {
			finalHosts[h] = struct{}{}
		}
	}
	return DomainMutationResult{
		ConfigID: t.ConfigID,
		Hosts:    sortedKeys(finalHosts),
		Changed:  changed,
		Added:    sortedKeys(addSet),
		Removed:  sortedKeys(removeSet),
	}, nil
}

func mutateUpstreamsForID(root map[string]any, t UpstreamMutationTarget) (UpstreamMutationResult, error) {
	cfg, ok := findConfigByID(root, strings.TrimSpace(t.ConfigID))
	if !ok {
		return UpstreamMutationResult{}, ErrConfigIDNotFound
	}
	addDial := strings.TrimSpace(t.AddDial)
	removeDial := strings.TrimSpace(t.RemoveDial)
	probeTimeout := t.ProbeTimeout
	if probeTimeout <= 0 {
		probeTimeout = 2 * time.Second
	}
	changed := false
	pruned := make([]string, 0)
	upstreams, refs := collectUpstreamRefs(cfg)
	for _, ref := range refs {
		current := upstreamDialSet(ref.Array)
		if addDial != "" {
			current[addDial] = struct{}{}
		}
		if removeDial != "" {
			delete(current, removeDial)
		}
		if t.PruneUnhealthy {
			for dial := range current {
				if !isDialReachable(dial, probeTimeout) {
					delete(current, dial)
					pruned = append(pruned, dial)
				}
			}
		}
		next := sortedKeys(current)
		prev := upstreamDialSlice(ref.Array)
		if !stringSlicesEqual(prev, next) {
			ref.Parent[ref.Key] = toUpstreamArray(next)
			changed = true
		}
	}
	finalSet := make(map[string]struct{}, len(upstreams))
	for _, d := range upstreams {
		finalSet[d] = struct{}{}
	}
	if changed {
		finalSet = make(map[string]struct{})
		for _, ref := range refs {
			for _, d := range upstreamDialSlice(ref.Parent[ref.Key]) {
				finalSet[d] = struct{}{}
			}
		}
	}
	sort.Strings(pruned)
	return UpstreamMutationResult{
		ConfigID:  t.ConfigID,
		Upstreams: sortedKeys(finalSet),
		Pruned:    uniqueSortedStrings(pruned),
		Changed:   changed,
		Added:     valueIfNonEmpty(addDial),
		Removed:   valueIfNonEmpty(removeDial),
	}, nil
}

func buildDomainDiff(results []DomainMutationResult) DomainMutationDiff {
	added := make(map[string]struct{})
	removed := make(map[string]struct{})
	for _, r := range results {
		for _, item := range r.Added {
			added[item] = struct{}{}
		}
		for _, item := range r.Removed {
			removed[item] = struct{}{}
		}
	}
	return DomainMutationDiff{Added: sortedKeys(added), Removed: sortedKeys(removed)}
}

func buildUpstreamDiff(results []UpstreamMutationResult) UpstreamMutationDiff {
	added := make(map[string]struct{})
	removed := make(map[string]struct{})
	pruned := make(map[string]struct{})
	for _, r := range results {
		for _, item := range r.Added {
			added[item] = struct{}{}
		}
		for _, item := range r.Removed {
			removed[item] = struct{}{}
		}
		for _, item := range r.Pruned {
			pruned[item] = struct{}{}
		}
	}
	return UpstreamMutationDiff{
		Added:   sortedKeys(added),
		Removed: sortedKeys(removed),
		Pruned:  sortedKeys(pruned),
	}
}

func buildPreviewByDomainTargets(root map[string]any, targets []DomainMutationTarget) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(targets))
	for _, t := range targets {
		if cfg, ok := findConfigByID(root, strings.TrimSpace(t.ConfigID)); ok {
			if b, err := json.Marshal(cfg); err == nil {
				out[t.ConfigID] = json.RawMessage(b)
			}
		}
	}
	return out
}

func buildPreviewByUpstreamTargets(root map[string]any, targets []UpstreamMutationTarget) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(targets))
	for _, t := range targets {
		if cfg, ok := findConfigByID(root, strings.TrimSpace(t.ConfigID)); ok {
			if b, err := json.Marshal(cfg); err == nil {
				out[t.ConfigID] = json.RawMessage(b)
			}
		}
	}
	return out
}

func valueIfNonEmpty(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return []string{strings.TrimSpace(s)}
}

func applyTLSPolicyChanges(root map[string]any, domains []string, challenge *TLSDNSChallenge, remove bool) (bool, error) {
	if len(domains) == 0 {
		return false, nil
	}
	tlsNode := ensureObjectPath(root, "apps", "tls", "automation")
	rawPolicies, ok := tlsNode["policies"].([]any)
	if !ok {
		rawPolicies = make([]any, 0)
	}
	policies := rawPolicies
	domainSet := toStringSet(domains)
	changed := false
	if remove {
		next := make([]any, 0, len(policies))
		for _, p := range policies {
			obj, ok := p.(map[string]any)
			if !ok {
				next = append(next, p)
				continue
			}
			subjects := toStringSet(anyToStringSlice(obj["subjects"]))
			before := len(subjects)
			for d := range domainSet {
				delete(subjects, d)
			}
			if before != len(subjects) {
				changed = true
			}
			if len(subjects) == 0 {
				changed = true
				continue
			}
			obj["subjects"] = sortedKeys(subjects)
			next = append(next, obj)
		}
		policies = next
	} else if challenge != nil {
		subjects := sortedKeys(domainSet)
		exists := false
		for _, p := range policies {
			obj, ok := p.(map[string]any)
			if !ok {
				continue
			}
			if stringSlicesEqual(anyToStringSlice(obj["subjects"]), subjects) {
				exists = true
				break
			}
		}
		if !exists {
			policies = append(policies, map[string]any{
				"subjects": subjects,
				"issuers": []any{
					map[string]any{
						"module": "acme",
						"challenges": map[string]any{
							"dns": map[string]any{
								"provider": map[string]any{
									"name":      strings.TrimSpace(challenge.Provider),
									"api_token": strings.TrimSpace(challenge.APIToken),
								},
							},
						},
					},
				},
			})
			changed = true
		}
	}
	if changed {
		tlsNode["policies"] = policies
	}
	return changed, nil
}

func configToMap(raw []byte) (map[string]any, error) {
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidMutationPayload, err)
	}
	return parsed, nil
}

func mapToConfig(parsed map[string]any) ([]byte, error) {
	b, err := json.Marshal(parsed)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func findConfigByID(v any, id string) (map[string]any, bool) {
	switch t := v.(type) {
	case map[string]any:
		if got, ok := t["@id"].(string); ok && strings.TrimSpace(got) == id {
			return t, true
		}
		for _, child := range t {
			if obj, ok := findConfigByID(child, id); ok {
				return obj, true
			}
		}
	case []any:
		for _, item := range t {
			if obj, ok := findConfigByID(item, id); ok {
				return obj, true
			}
		}
	}
	return nil, false
}

func toStringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		out[v] = struct{}{}
	}
	return out
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func anyToStringSlice(v any) []string {
	list, ok := v.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		if !ok {
			continue
		}
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return uniqueSortedStrings(out)
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	a2 := append([]string(nil), a...)
	b2 := append([]string(nil), b...)
	sort.Strings(a2)
	sort.Strings(b2)
	for i := range a2 {
		if a2[i] != b2[i] {
			return false
		}
	}
	return true
}

type upstreamRef struct {
	Parent map[string]any
	Key    string
	Array  []any
}

func collectUpstreamRefs(v any) ([]string, []upstreamRef) {
	dials := make([]string, 0)
	refs := make([]upstreamRef, 0)
	var walk func(any)
	walk = func(node any) {
		switch t := node.(type) {
		case map[string]any:
			for k, child := range t {
				if k == "upstreams" {
					if arr, ok := child.([]any); ok {
						refs = append(refs, upstreamRef{Parent: t, Key: k, Array: arr})
						dials = append(dials, upstreamDialSlice(arr)...)
					}
				}
				walk(child)
			}
		case []any:
			for _, item := range t {
				walk(item)
			}
		}
	}
	walk(v)
	return uniqueSortedStrings(dials), refs
}

func upstreamDialSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		dial, ok := obj["dial"].(string)
		if !ok || strings.TrimSpace(dial) == "" {
			continue
		}
		out = append(out, strings.TrimSpace(dial))
	}
	return uniqueSortedStrings(out)
}

func upstreamDialSet(v any) map[string]struct{} {
	return toStringSet(upstreamDialSlice(v))
}

func toUpstreamArray(dials []string) []any {
	out := make([]any, 0, len(dials))
	for _, dial := range dials {
		out = append(out, map[string]any{"dial": dial})
	}
	return out
}

func isDialReachable(dial string, timeout time.Duration) bool {
	dial = strings.TrimSpace(dial)
	if dial == "" {
		return false
	}
	conn, err := net.DialTimeout("tcp", dial, timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func ensureObjectPath(root map[string]any, keys ...string) map[string]any {
	current := root
	for _, key := range keys {
		next, ok := current[key].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[key] = next
		}
		current = next
	}
	return current
}
