package secrets

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	awssvc "github.com/nicholasricci/caddy-dashboard/internal/aws"
)

// Resolver resolves secret references like file://, env://, aws-secrets://arn...
type Resolver interface {
	Resolve(ctx context.Context, ref string) ([]byte, error)
}

// MultiResolver implements Resolver with URI-style refs.
type MultiResolver struct {
	secrets *awssvc.SecretsService
	ttl     time.Duration

	mu      sync.Mutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	value []byte
	until time.Time
}

// NewResolver builds a resolver. secrets may be nil (aws-secrets:// will error).
func NewResolver(secrets *awssvc.SecretsService, cacheTTL time.Duration) *MultiResolver {
	if cacheTTL <= 0 {
		cacheTTL = 5 * time.Minute
	}
	return &MultiResolver{
		secrets: secrets,
		ttl:     cacheTTL,
		entries: make(map[string]cacheEntry),
	}
}

func (r *MultiResolver) Resolve(ctx context.Context, ref string) ([]byte, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("empty secret ref")
	}

	r.mu.Lock()
	if ent, ok := r.entries[ref]; ok && time.Now().Before(ent.until) {
		cp := append([]byte(nil), ent.value...)
		r.mu.Unlock()
		return cp, nil
	}
	r.mu.Unlock()

	var b []byte
	var err error
	switch {
	case strings.HasPrefix(ref, "file://"):
		b, err = resolveFileRef(ref)
	case strings.HasPrefix(ref, "env://"):
		b, err = resolveEnvRef(ref)
	case strings.HasPrefix(ref, "aws-secrets://"):
		b, err = r.resolveAWSSecrets(ctx, ref)
	default:
		return nil, fmt.Errorf("unsupported secret ref scheme (use file://, env://, aws-secrets://): %q", ref)
	}
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.entries[ref] = cacheEntry{value: append([]byte(nil), b...), until: time.Now().Add(r.ttl)}
	r.mu.Unlock()
	return b, nil
}

func resolveFileRef(ref string) ([]byte, error) {
	path := strings.TrimPrefix(ref, "file://")
	path = strings.TrimPrefix(path, "//") // file:///etc/foo
	if path == "" {
		return nil, fmt.Errorf("invalid file ref %q", ref)
	}
	return os.ReadFile(path)
}

func resolveEnvRef(ref string) ([]byte, error) {
	name := strings.TrimPrefix(ref, "env://")
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("invalid env ref %q", ref)
	}
	v, ok := os.LookupEnv(name)
	if !ok {
		return nil, fmt.Errorf("env var %q not set", name)
	}
	return []byte(v), nil
}

func (r *MultiResolver) resolveAWSSecrets(ctx context.Context, ref string) ([]byte, error) {
	if r == nil || r.secrets == nil {
		return nil, fmt.Errorf("aws secrets resolver not configured")
	}
	arn := strings.TrimPrefix(ref, "aws-secrets://")
	arn = strings.TrimSpace(arn)
	region, err := regionFromSecretsARN(arn)
	if err != nil {
		return nil, err
	}
	s, err := r.secrets.GetSecretString(ctx, region, arn)
	if err != nil {
		return nil, err
	}
	return []byte(s), nil
}

func regionFromSecretsARN(arn string) (string, error) {
	parts := strings.Split(arn, ":")
	if len(parts) < 4 || parts[0] != "arn" || parts[1] != "aws" {
		return "", fmt.Errorf("invalid secrets arn")
	}
	if parts[2] != "secretsmanager" {
		return "", fmt.Errorf("arn is not secretsmanager")
	}
	return parts[3], nil
}
