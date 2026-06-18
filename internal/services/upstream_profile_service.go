package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
)

var (
	ErrUpstreamProfileNotFound       = errors.New("upstream profile not found")
	ErrUpstreamProfileNameRequired   = errors.New("upstream profile name is required")
	ErrUpstreamProfileBindingsEmpty  = errors.New("at least one binding is required")
	ErrUpstreamProfileNameTaken      = errors.New("upstream profile name already exists for discovery group")
	ErrUpstreamProfileInvalidBinding = errors.New("invalid upstream profile binding")
)

type UpstreamProfileService struct {
	profiles    *repository.UpstreamProfileRepository
	discoveries *repository.DiscoveryRepository
}

func NewUpstreamProfileService(
	profiles *repository.UpstreamProfileRepository,
	discoveries *repository.DiscoveryRepository,
) *UpstreamProfileService {
	return &UpstreamProfileService{profiles: profiles, discoveries: discoveries}
}

type CreateUpstreamProfileInput struct {
	DiscoveryConfigID uuid.UUID
	Name              string
	Description       string
	Bindings          []models.UpstreamProfileBinding
}

type UpdateUpstreamProfileInput struct {
	Name        string
	Description string
	Bindings    []models.UpstreamProfileBinding
}

func (s *UpstreamProfileService) ListByDiscovery(ctx context.Context, discoveryID uuid.UUID) ([]models.UpstreamProfile, error) {
	if _, err := s.discoveries.GetByID(ctx, discoveryID); err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrDiscoveryNotFound
		}
		return nil, err
	}
	return s.profiles.ListByDiscoveryConfigID(ctx, discoveryID)
}

func (s *UpstreamProfileService) Get(ctx context.Context, id uuid.UUID) (*models.UpstreamProfile, error) {
	profile, err := s.profiles.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrUpstreamProfileNotFound
		}
		return nil, err
	}
	return profile, nil
}

func (s *UpstreamProfileService) Create(ctx context.Context, in CreateUpstreamProfileInput) (*models.UpstreamProfile, error) {
	if _, err := s.discoveries.GetByID(ctx, in.DiscoveryConfigID); err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrDiscoveryNotFound
		}
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, ErrUpstreamProfileNameRequired
	}
	bindings, err := normalizeBindings(in.Bindings)
	if err != nil {
		return nil, err
	}
	if _, err := s.profiles.GetByNameAndDiscovery(ctx, in.DiscoveryConfigID, name); err == nil {
		return nil, ErrUpstreamProfileNameTaken
	} else if !repository.IsNotFound(err) {
		return nil, err
	}
	bindingsJSON, err := json.Marshal(bindings)
	if err != nil {
		return nil, err
	}
	profile := &models.UpstreamProfile{
		DiscoveryConfigID: in.DiscoveryConfigID,
		Name:              name,
		Description:       strings.TrimSpace(in.Description),
		Bindings:          bindingsJSON,
	}
	if err := s.profiles.Create(ctx, profile); err != nil {
		return nil, err
	}
	return profile, nil
}

func (s *UpstreamProfileService) Update(ctx context.Context, id uuid.UUID, in UpdateUpstreamProfileInput) (*models.UpstreamProfile, error) {
	profile, err := s.profiles.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrUpstreamProfileNotFound
		}
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, ErrUpstreamProfileNameRequired
	}
	bindings, err := normalizeBindings(in.Bindings)
	if err != nil {
		return nil, err
	}
	if existing, err := s.profiles.GetByNameAndDiscovery(ctx, profile.DiscoveryConfigID, name); err == nil && existing.ID != profile.ID {
		return nil, ErrUpstreamProfileNameTaken
	} else if err != nil && !repository.IsNotFound(err) {
		return nil, err
	}
	bindingsJSON, err := json.Marshal(bindings)
	if err != nil {
		return nil, err
	}
	profile.Name = name
	profile.Description = strings.TrimSpace(in.Description)
	profile.Bindings = bindingsJSON
	if err := s.profiles.Update(ctx, profile); err != nil {
		return nil, err
	}
	return profile, nil
}

func (s *UpstreamProfileService) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.profiles.GetByID(ctx, id); err != nil {
		if repository.IsNotFound(err) {
			return ErrUpstreamProfileNotFound
		}
		return err
	}
	return s.profiles.Delete(ctx, id)
}

// ParseBindings unmarshals profile bindings JSON.
func ParseBindings(raw json.RawMessage) ([]models.UpstreamProfileBinding, error) {
	if len(raw) == 0 {
		return nil, ErrUpstreamProfileBindingsEmpty
	}
	var bindings []models.UpstreamProfileBinding
	if err := json.Unmarshal(raw, &bindings); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstreamProfileInvalidBinding, err)
	}
	return normalizeBindings(bindings)
}

func normalizeBindings(bindings []models.UpstreamProfileBinding) ([]models.UpstreamProfileBinding, error) {
	if len(bindings) == 0 {
		return nil, ErrUpstreamProfileBindingsEmpty
	}
	out := make([]models.UpstreamProfileBinding, 0, len(bindings))
	for _, b := range bindings {
		configID := strings.TrimSpace(b.ConfigID)
		if configID == "" {
			return nil, fmt.Errorf("%w: config_id is required", ErrUpstreamProfileInvalidBinding)
		}
		if b.Port <= 0 || b.Port > 65535 {
			return nil, fmt.Errorf("%w: port must be 1..65535 for config_id %q", ErrUpstreamProfileInvalidBinding, configID)
		}
		out = append(out, models.UpstreamProfileBinding{ConfigID: configID, Port: b.Port})
	}
	return out, nil
}
