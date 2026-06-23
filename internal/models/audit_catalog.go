package models

var auditActions = []string{
	"apply",
	"backfill",
	"create",
	"delete",
	"mutate_domains",
	"mutate_upstreams",
	"propagate",
	"register_domain",
	"register_domain_profile",
	"register_upstream",
	"register_upstream_profile",
	"reload",
	"revoke",
	"run",
	"sync",
	"update",
}

var auditResources = []string{
	"api_key",
	"discovery",
	"domain_profile",
	"node",
	"snapshot",
	"upstream_profile",
	"user",
}

var auditActionSet map[string]struct{}
var auditResourceSet map[string]struct{}

func init() {
	auditActionSet = make(map[string]struct{}, len(auditActions))
	for _, a := range auditActions {
		auditActionSet[a] = struct{}{}
	}
	auditResourceSet = make(map[string]struct{}, len(auditResources))
	for _, r := range auditResources {
		auditResourceSet[r] = struct{}{}
	}
}

func AuditActions() []string {
	out := make([]string, len(auditActions))
	copy(out, auditActions)
	return out
}

func AuditResources() []string {
	out := make([]string, len(auditResources))
	copy(out, auditResources)
	return out
}

func IsValidAuditAction(s string) bool {
	_, ok := auditActionSet[s]
	return ok
}

func IsValidAuditResource(s string) bool {
	_, ok := auditResourceSet[s]
	return ok
}
