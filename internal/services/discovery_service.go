package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	awssvc "github.com/nicholasricci/caddy-dashboard/internal/aws"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
)

var (
	ErrDiscoveryNotFound             = errors.New("discovery config not found")
	ErrDiscoveryMethodNotImplemented = errors.New("discovery method not implemented yet")
	ErrDiscoveryUnknownMethod        = errors.New("unknown discovery method")
)

// discoveryRepository is satisfied by *repository.DiscoveryRepository; narrowed for tests.
type discoveryRepository interface {
	List(ctx context.Context) ([]models.DiscoveryConfig, error)
	ListPaginated(ctx context.Context, limit, offset int) ([]models.DiscoveryConfig, int64, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.DiscoveryConfig, error)
	Create(ctx context.Context, cfg *models.DiscoveryConfig) error
	Update(ctx context.Context, cfg *models.DiscoveryConfig) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type discoveryNodeWriter interface {
	UpsertByInstanceOrIP(ctx context.Context, node *models.CaddyNode) error
}

type DiscoveryService struct {
	repo     discoveryRepository
	nodeRepo discoveryNodeWriter
	ec2      *awssvc.EC2Service
	ssm      *awssvc.SSMService
}

func NewDiscoveryService(repo discoveryRepository, nodeRepo discoveryNodeWriter, ec2 *awssvc.EC2Service, ssm *awssvc.SSMService) *DiscoveryService {
	return &DiscoveryService{
		repo:     repo,
		nodeRepo: nodeRepo,
		ec2:      ec2,
		ssm:      ssm,
	}
}

func (s *DiscoveryService) List(ctx context.Context) ([]models.DiscoveryConfig, error) {
	return s.repo.List(ctx)
}

func (s *DiscoveryService) ListPaginated(ctx context.Context, limit, offset int) ([]models.DiscoveryConfig, int64, error) {
	return s.repo.ListPaginated(ctx, limit, offset)
}

func (s *DiscoveryService) Get(ctx context.Context, id uuid.UUID) (*models.DiscoveryConfig, error) {
	cfg, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrDiscoveryNotFound
		}
		return nil, err
	}
	return cfg, nil
}

func (s *DiscoveryService) Create(ctx context.Context, cfg *models.DiscoveryConfig) error {
	s.normalize(cfg)
	return s.repo.Create(ctx, cfg)
}

func (s *DiscoveryService) Update(ctx context.Context, cfg *models.DiscoveryConfig) error {
	s.normalize(cfg)
	if _, err := s.repo.GetByID(ctx, cfg.ID); err != nil {
		if repository.IsNotFound(err) {
			return ErrDiscoveryNotFound
		}
		return err
	}
	return s.repo.Update(ctx, cfg)
}

func (s *DiscoveryService) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.repo.GetByID(ctx, id); err != nil {
		if repository.IsNotFound(err) {
			return ErrDiscoveryNotFound
		}
		return err
	}
	return s.repo.Delete(ctx, id)
}

func (s *DiscoveryService) normalize(cfg *models.DiscoveryConfig) {
	cfg.Method = strings.TrimSpace(cfg.Method)
	if cfg.Method == "" {
		cfg.Method = models.DiscoveryMethodAWSTag
	}
}

func (s *DiscoveryService) Run(ctx context.Context, id uuid.UUID) (int, error) {
	cfg, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return 0, ErrDiscoveryNotFound
		}
		return 0, err
	}
	s.normalize(cfg)

	var nodes []models.CaddyNode
	switch cfg.Method {
	case models.DiscoveryMethodAWSTag:
		nodes, err = s.ec2.DiscoverByTag(ctx, cfg.Region, cfg.TagKey, cfg.TagValue)
		if err != nil {
			return 0, err
		}
	case models.DiscoveryMethodAWSSSM:
		nodes, err = s.ssm.DiscoverManagedInstances(ctx, cfg.Region)
		if err != nil {
			return 0, err
		}
	case models.DiscoveryMethodStaticIP:
		nodes, err = staticIPNodes(cfg)
		if err != nil {
			return 0, err
		}
	case models.DiscoveryMethodAWSCIDR:
		return 0, fmt.Errorf("%w: aws_cidr", ErrDiscoveryMethodNotImplemented)
	default:
		return 0, fmt.Errorf("%w: %q", ErrDiscoveryUnknownMethod, cfg.Method)
	}

	for i := range nodes {
		n := nodes[i]
		if err := s.nodeRepo.UpsertByInstanceOrIP(ctx, &n); err != nil {
			return 0, err
		}
	}
	return len(nodes), nil
}

type staticDiscoveryParams struct {
	Endpoints []struct {
		PrivateIP string `json:"private_ip"`
		Name      string `json:"name"`
		Region    string `json:"region"`
	} `json:"endpoints"`
}

func staticIPNodes(cfg *models.DiscoveryConfig) ([]models.CaddyNode, error) {
	var p staticDiscoveryParams
	if len(cfg.Parameters) == 0 {
		return nil, fmt.Errorf("static_ip discovery requires parameters with endpoints array")
	}
	if err := json.Unmarshal(cfg.Parameters, &p); err != nil {
		return nil, fmt.Errorf("invalid parameters json: %w", err)
	}
	if len(p.Endpoints) == 0 {
		return nil, fmt.Errorf("static_ip discovery requires at least one endpoint")
	}
	now := time.Now().UTC()
	out := make([]models.CaddyNode, 0, len(p.Endpoints))
	for _, e := range p.Endpoints {
		ip := strings.TrimSpace(e.PrivateIP)
		if ip == "" {
			continue
		}
		region := strings.TrimSpace(e.Region)
		if region == "" {
			region = cfg.Region
		}
		name := strings.TrimSpace(e.Name)
		if name == "" {
			name = ip
		}
		ipCopy := ip
		out = append(out, models.CaddyNode{
			Name:       name,
			PrivateIP:  &ipCopy,
			Region:     region,
			SSMEnabled: true,
			Status:     "static",
			LastSeenAt: &now,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid private_ip entries in parameters.endpoints")
	}
	return out, nil
}
