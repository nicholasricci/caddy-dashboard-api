package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/auth"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
)

var (
	ErrAPIKeyNotFound                 = errors.New("api key not found")
	ErrAPIKeyInvalid                  = errors.New("invalid api key")
	ErrAPIKeyRevoked                  = errors.New("api key revoked")
	ErrAPIKeyExpired                  = errors.New("api key expired")
	ErrAPIKeyNameRequired             = errors.New("api key name is required")
	ErrAPIKeyScopeRequired            = errors.New("at least one scope is required")
	ErrAPIKeyDiscoveryEmpty           = errors.New("at least one allowed discovery config id is required")
	ErrAPIKeyForbidden                = errors.New("api key not authorized for discovery config")
	ErrAPIKeyProfileForbidden         = errors.New("api key not authorized for upstream profile")
	ErrAPIKeyProfileDiscoveryMismatch = errors.New("upstream profile discovery not in allowed discovery config ids")
	ErrAPIKeyProfileNotFound          = errors.New("upstream profile not found")
	ErrAPIKeyScopeMissing             = errors.New("api key missing required scope")
)

// ValidatedAPIKey is attached to request context after successful API key auth.
type ValidatedAPIKey struct {
	ID                        uuid.UUID
	Name                      string
	Scopes                    []string
	AllowedDiscoveryConfigIDs []uuid.UUID
	AllowedUpstreamProfileIDs []uuid.UUID
}

type APIKeyService struct {
	repo     *repository.APIKeyRepository
	profiles *repository.UpstreamProfileRepository
}

func NewAPIKeyService(repo *repository.APIKeyRepository, profiles *repository.UpstreamProfileRepository) *APIKeyService {
	return &APIKeyService{repo: repo, profiles: profiles}
}

type CreateAPIKeyInput struct {
	Name                      string
	Scopes                    []string
	AllowedDiscoveryConfigIDs []uuid.UUID
	AllowedUpstreamProfileIDs []uuid.UUID
	ExpiresAt                 *time.Time
}

func (s *APIKeyService) List(ctx context.Context) ([]models.APIKey, error) {
	return s.repo.List(ctx)
}

func (s *APIKeyService) Get(ctx context.Context, id uuid.UUID) (*models.APIKey, error) {
	key, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, err
	}
	return key, nil
}

func (s *APIKeyService) Create(ctx context.Context, in CreateAPIKeyInput) (*models.APIKeyCreateResponse, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, ErrAPIKeyNameRequired
	}
	scopes := normalizeScopes(in.Scopes)
	if len(scopes) == 0 {
		return nil, ErrAPIKeyScopeRequired
	}
	allowed := uniqueUUIDs(in.AllowedDiscoveryConfigIDs)
	if len(allowed) == 0 {
		return nil, ErrAPIKeyDiscoveryEmpty
	}
	profileIDs := uniqueUUIDs(in.AllowedUpstreamProfileIDs)
	if err := s.validateAllowedProfiles(ctx, allowed, profileIDs); err != nil {
		return nil, err
	}
	scopesJSON, err := json.Marshal(scopes)
	if err != nil {
		return nil, err
	}
	allowedJSON, err := json.Marshal(uuidStrings(allowed))
	if err != nil {
		return nil, err
	}
	profilesJSON, err := json.Marshal(uuidStrings(profileIDs))
	if err != nil {
		return nil, err
	}
	secret, hash, prefix, err := auth.GenerateAPIKeySecret()
	if err != nil {
		return nil, err
	}
	key := &models.APIKey{
		Name:                      name,
		KeyPrefix:                 prefix,
		KeyHash:                   hash,
		Scopes:                    scopesJSON,
		AllowedDiscoveryConfigIDs: allowedJSON,
		AllowedUpstreamProfileIDs: profilesJSON,
		ExpiresAt:                 in.ExpiresAt,
	}
	if err := s.repo.Create(ctx, key); err != nil {
		return nil, err
	}
	return &models.APIKeyCreateResponse{APIKey: *key, Secret: secret}, nil
}

func (s *APIKeyService) Revoke(ctx context.Context, id uuid.UUID) error {
	key, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return ErrAPIKeyNotFound
		}
		return err
	}
	if key.RevokedAt != nil {
		return nil
	}
	now := time.Now().UTC()
	return s.repo.Revoke(ctx, id, now)
}

func (s *APIKeyService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	return nil
}

func (s *APIKeyService) Validate(ctx context.Context, plaintext string) (*ValidatedAPIKey, error) {
	plain := strings.TrimSpace(plaintext)
	if plain == "" || !auth.IsAPIKeyToken(plain) {
		return nil, ErrAPIKeyInvalid
	}
	hash := auth.HashAPIKey(plain)
	key, err := s.repo.GetByKeyHash(ctx, hash)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrAPIKeyInvalid
		}
		return nil, err
	}
	now := time.Now().UTC()
	if key.RevokedAt != nil {
		return nil, ErrAPIKeyRevoked
	}
	if key.ExpiresAt != nil && now.After(*key.ExpiresAt) {
		return nil, ErrAPIKeyExpired
	}
	scopes, err := parseStringSliceJSON(key.Scopes)
	if err != nil {
		return nil, err
	}
	allowedStr, err := parseStringSliceJSON(key.AllowedDiscoveryConfigIDs)
	if err != nil {
		return nil, err
	}
	allowed := parseUUIDSlice(allowedStr)
	profilesStr, err := parseStringSliceJSON(key.AllowedUpstreamProfileIDs)
	if err != nil {
		return nil, err
	}
	profiles := parseUUIDSlice(profilesStr)
	_ = s.repo.TouchLastUsed(ctx, key.ID, now)
	return &ValidatedAPIKey{
		ID:                        key.ID,
		Name:                      key.Name,
		Scopes:                    scopes,
		AllowedDiscoveryConfigIDs: allowed,
		AllowedUpstreamProfileIDs: profiles,
	}, nil
}

func (s *APIKeyService) AuthorizeDiscovery(validated *ValidatedAPIKey, discoveryConfigID uuid.UUID, requiredScope string) error {
	if validated == nil {
		return ErrAPIKeyInvalid
	}
	if !containsString(validated.Scopes, requiredScope) {
		return ErrAPIKeyScopeMissing
	}
	for _, id := range validated.AllowedDiscoveryConfigIDs {
		if id == discoveryConfigID {
			return nil
		}
	}
	return ErrAPIKeyForbidden
}

func (s *APIKeyService) AuthorizeUpstreamProfile(validated *ValidatedAPIKey, profileID uuid.UUID, discoveryConfigID uuid.UUID, requiredScope string) error {
	if validated == nil {
		return ErrAPIKeyInvalid
	}
	if !containsString(validated.Scopes, requiredScope) {
		return ErrAPIKeyScopeMissing
	}
	if !containsUUID(validated.AllowedUpstreamProfileIDs, profileID) {
		return ErrAPIKeyProfileForbidden
	}
	if !containsUUID(validated.AllowedDiscoveryConfigIDs, discoveryConfigID) {
		return ErrAPIKeyForbidden
	}
	return nil
}

func (s *APIKeyService) validateAllowedProfiles(ctx context.Context, allowedDiscoveries, profileIDs []uuid.UUID) error {
	if len(profileIDs) == 0 || s.profiles == nil {
		return nil
	}
	profiles, err := s.profiles.GetByIDs(ctx, profileIDs)
	if err != nil {
		return err
	}
	found := make(map[uuid.UUID]struct{}, len(profiles))
	for _, p := range profiles {
		found[p.ID] = struct{}{}
		if !containsUUID(allowedDiscoveries, p.DiscoveryConfigID) {
			return ErrAPIKeyProfileDiscoveryMismatch
		}
	}
	for _, id := range profileIDs {
		if _, ok := found[id]; !ok {
			return ErrAPIKeyProfileNotFound
		}
	}
	return nil
}

func normalizeScopes(scopes []string) []string {
	out := make([]string, 0, len(scopes))
	seen := make(map[string]struct{}, len(scopes))
	for _, s := range scopes {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func uniqueUUIDs(in []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(in))
	out := make([]uuid.UUID, 0, len(in))
	for _, id := range in {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

func parseStringSliceJSON(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func containsUUID(items []uuid.UUID, want uuid.UUID) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func parseUUIDSlice(raw []string) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(raw))
	for _, item := range raw {
		id, err := uuid.Parse(strings.TrimSpace(item))
		if err != nil {
			continue
		}
		out = append(out, id)
	}
	return out
}
