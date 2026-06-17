package caddy

import (
	"testing"
	"time"
)

func TestMutateHostsForID_AddRemove(t *testing.T) {
	root := map[string]any{
		"apps": map[string]any{
			"http": map[string]any{
				"servers": map[string]any{
					"srv0": map[string]any{
						"routes": []any{
							map[string]any{
								"@id": "route-a",
								"match": []any{
									map[string]any{"host": []any{"a.example.com", "b.example.com"}},
								},
							},
						},
					},
				},
			},
		},
	}
	res, err := mutateHostsForID(root, DomainMutationTarget{
		ConfigID:      "route-a",
		MatchIndexes:  []int{0},
		AddDomains:    []string{"c.example.com", "a.example.com"},
		RemoveDomains: []string{"b.example.com"},
	})
	if err != nil {
		t.Fatalf("mutateHostsForID: %v", err)
	}
	if !res.Changed {
		t.Fatal("expected changed")
	}
	if len(res.Hosts) != 2 {
		t.Fatalf("hosts=%v", res.Hosts)
	}
}

func TestMutateUpstreamsForID_AddRemove(t *testing.T) {
	root := map[string]any{
		"@id": "route-a",
		"handle": []any{
			map[string]any{
				"handler": "reverse_proxy",
				"upstreams": []any{
					map[string]any{"dial": "10.0.0.1:80"},
				},
			},
		},
	}
	res, err := mutateUpstreamsForID(root, UpstreamMutationTarget{
		ConfigID:   "route-a",
		AddDial:    "10.0.0.2:80",
		RemoveDial: "10.0.0.1:80",
	})
	if err != nil {
		t.Fatalf("mutateUpstreamsForID: %v", err)
	}
	if !res.Changed {
		t.Fatal("expected changed")
	}
	if len(res.Upstreams) != 1 || res.Upstreams[0] != "10.0.0.2:80" {
		t.Fatalf("upstreams=%v", res.Upstreams)
	}
}

func TestMutateUpstreamsForID_PruneUnhealthy(t *testing.T) {
	root := map[string]any{
		"@id": "route-a",
		"handle": []any{
			map[string]any{
				"upstreams": []any{
					map[string]any{"dial": "127.0.0.1:1"},
				},
			},
		},
	}
	res, err := mutateUpstreamsForID(root, UpstreamMutationTarget{
		ConfigID:       "route-a",
		PruneUnhealthy: true,
		ProbeTimeout:   20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("mutateUpstreamsForID: %v", err)
	}
	if !res.Changed {
		t.Fatal("expected changed")
	}
	if len(res.Upstreams) != 0 {
		t.Fatalf("upstreams=%v", res.Upstreams)
	}
}
