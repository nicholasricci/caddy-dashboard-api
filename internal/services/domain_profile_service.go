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
	ErrDomainProfileNotFound       = errors.New("domain profile not found")
	ErrDomainProfileNameRequired   = errors.New("domain profile name is required")
	ErrDomainProfileBindingsEmpty  = errors.New("at least one binding is required")
	ErrDomainProfileNameTaken      = errors.New("domain profile name already exists for discovery group")
	ErrDomainProfileInvalidBinding = errors.New("invalid domain profile binding")
)

type DomainProfileService struct {
	profiles    *repository.DomainProfileRepository
	discoveries *repository.DiscoveryRepository
}

func NewDomainProfileService(
	profiles *repository.DomainProfileRepository,
	discoveries *repository.DiscoveryRepository,
) *DomainProfileService {
	return &DomainProfileService{profiles: profiles, discoveries: discoveries}
}

type CreateDomainProfileInput struct {
	DiscoveryConfigID uuid.UUID
	Name              string
	Description       string
	Bindings          []models.DomainProfileBinding
}

type UpdateDomainProfileInput struct {
	Name        string
	Description string
	Bindings    []models.DomainProfileBinding
}

func (s *DomainProfileService) ListByDiscovery(ctx context.Context, discoveryID uuid.UUID) ([]models.DomainProfile, error) {
	if _, err := s.discoveries.GetByID(ctx, discoveryID); err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrDiscoveryNotFound
		}
		return nil, err
	}
	return s.profiles.ListByDiscoveryConfigID(ctx, discoveryID)
}

func (s *DomainProfileService) Get(ctx context.Context, id uuid.UUID) (*models.DomainProfile, error) {
	profile, err := s.profiles.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrDomainProfileNotFound
		}
		return nil, err
	}
	return profile, nil
}

func (s *DomainProfileService) Create(ctx context.Context, in CreateDomainProfileInput) (*models.DomainProfile, error) {
	if _, err := s.discoveries.GetByID(ctx, in.DiscoveryConfigID); err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrDiscoveryNotFound
		}
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, ErrDomainProfileNameRequired
	}
	bindings, err := normalizeDomainBindings(in.Bindings)
	if err != nil {
		return nil, err
	}
	if _, err := s.profiles.GetByNameAndDiscovery(ctx, in.DiscoveryConfigID, name); err == nil {
		return nil, ErrDomainProfileNameTaken
	} else if !repository.IsNotFound(err) {
		return nil, err
	}
	bindingsJSON, err := json.Marshal(bindings)
	if err != nil {
		return nil, err
	}
	profile := &models.DomainProfile{
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

func (s *DomainProfileService) Update(ctx context.Context, id uuid.UUID, in UpdateDomainProfileInput) (*models.DomainProfile, error) {
	profile, err := s.profiles.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrDomainProfileNotFound
		}
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, ErrDomainProfileNameRequired
	}
	bindings, err := normalizeDomainBindings(in.Bindings)
	if err != nil {
		return nil, err
	}
	if existing, err := s.profiles.GetByNameAndDiscovery(ctx, profile.DiscoveryConfigID, name); err == nil && existing.ID != profile.ID {
		return nil, ErrDomainProfileNameTaken
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

func (s *DomainProfileService) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.profiles.GetByID(ctx, id); err != nil {
		if repository.IsNotFound(err) {
			return ErrDomainProfileNotFound
		}
		return err
	}
	return s.profiles.Delete(ctx, id)
}

// ParseDomainBindings unmarshals profile bindings JSON.
func ParseDomainBindings(raw json.RawMessage) ([]models.DomainProfileBinding, error) {
	if len(raw) == 0 {
		return nil, ErrDomainProfileBindingsEmpty
	}
	var bindings []models.DomainProfileBinding
	if err := json.Unmarshal(raw, &bindings); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDomainProfileInvalidBinding, err)
	}
	return normalizeDomainBindings(bindings)
}

func normalizeDomainBindings(bindings []models.DomainProfileBinding) ([]models.DomainProfileBinding, error) {
	if len(bindings) == 0 {
		return nil, ErrDomainProfileBindingsEmpty
	}
	out := make([]models.DomainProfileBinding, 0, len(bindings))
	for _, b := range bindings {
		configID := strings.TrimSpace(b.ConfigID)
		if configID == "" {
			return nil, fmt.Errorf("%w: config_id is required", ErrDomainProfileInvalidBinding)
		}
		indexes := b.MatchIndexes
		if len(indexes) == 0 {
			indexes = []int{0}
		}
		for _, idx := range indexes {
			if idx < 0 {
				return nil, fmt.Errorf("%w: match index must be >= 0 for config_id %q", ErrDomainProfileInvalidBinding, configID)
			}
		}
		out = append(out, models.DomainProfileBinding{ConfigID: configID, MatchIndexes: indexes})
	}
	return out, nil
}
