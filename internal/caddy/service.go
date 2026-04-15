package caddy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
	"gorm.io/datatypes"
)

// ErrNodeNoInstanceID is returned when a node cannot be reached via SSM (missing EC2 instance id).
var ErrNodeNoInstanceID = errors.New("node has no instance_id configured")

// nodeLoader is satisfied by *repository.NodeRepository; narrowed for tests.
type nodeLoader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.CaddyNode, error)
}

type snapshotWriter interface {
	Create(ctx context.Context, s *models.CaddySnapshot) error
}

type Service struct {
	nodes     nodeLoader
	snapshots snapshotWriter
	executor  *SSMExecutor
}

func NewService(nodes nodeLoader, snapshots snapshotWriter, executor *SSMExecutor) *Service {
	return &Service{
		nodes:     nodes,
		snapshots: snapshots,
		executor:  executor,
	}
}

func (s *Service) SyncNodeConfig(ctx context.Context, nodeID uuid.UUID, requestedBy string) error {
	node, err := s.nodes.GetByID(ctx, nodeID)
	if err != nil {
		if repository.IsNotFound(err) {
			return models.ErrNodeNotFound
		}
		return err
	}
	if node.InstanceID == nil || *node.InstanceID == "" {
		return ErrNodeNoInstanceID
	}
	res, err := s.executor.FetchConfig(ctx, node.Region, *node.InstanceID)
	if err != nil {
		return err
	}
	if res.Status != "Success" {
		return fmt.Errorf("ssm command status: %s: %s", res.Status, res.Stderr)
	}
	return s.storeSnapshot(ctx, nodeID, []byte(res.Stdout), requestedBy)
}

// GetLiveConfig returns the current Caddy JSON config from the node admin API (same SSM fetch as sync) without persisting a snapshot.
func (s *Service) GetLiveConfig(ctx context.Context, nodeID uuid.UUID) (json.RawMessage, error) {
	node, err := s.nodes.GetByID(ctx, nodeID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, models.ErrNodeNotFound
		}
		return nil, err
	}
	if node.InstanceID == nil || *node.InstanceID == "" {
		return nil, ErrNodeNoInstanceID
	}
	res, err := s.executor.FetchConfig(ctx, node.Region, *node.InstanceID)
	if err != nil {
		return nil, err
	}
	if res.Status != "Success" {
		return nil, fmt.Errorf("ssm command status: %s: %s", res.Status, res.Stderr)
	}
	out := []byte(strings.TrimSpace(res.Stdout))
	if len(out) == 0 {
		return nil, fmt.Errorf("empty config response from node")
	}
	var compact json.RawMessage
	if err := json.Unmarshal(out, &compact); err != nil {
		return nil, fmt.Errorf("config response is not valid json: %w", err)
	}
	return compact, nil
}

func (s *Service) ApplyConfig(ctx context.Context, nodeID uuid.UUID, payload []byte, requestedBy string) error {
	node, err := s.nodes.GetByID(ctx, nodeID)
	if err != nil {
		if repository.IsNotFound(err) {
			return models.ErrNodeNotFound
		}
		return err
	}
	if node.InstanceID == nil || *node.InstanceID == "" {
		return ErrNodeNoInstanceID
	}
	res, err := s.executor.ApplyConfig(ctx, node.Region, *node.InstanceID, payload)
	if err != nil {
		return err
	}
	if res.Status != "Success" {
		return fmt.Errorf("ssm command status: %s: %s", res.Status, res.Stderr)
	}
	return s.storeSnapshot(ctx, nodeID, payload, requestedBy)
}

func (s *Service) Reload(ctx context.Context, nodeID uuid.UUID) error {
	node, err := s.nodes.GetByID(ctx, nodeID)
	if err != nil {
		if repository.IsNotFound(err) {
			return models.ErrNodeNotFound
		}
		return err
	}
	if node.InstanceID == nil || *node.InstanceID == "" {
		return ErrNodeNoInstanceID
	}
	res, err := s.executor.Reload(ctx, node.Region, *node.InstanceID)
	if err != nil {
		return err
	}
	if res.Status != "Success" {
		return fmt.Errorf("ssm command status: %s: %s", res.Status, res.Stderr)
	}
	return nil
}

func (s *Service) storeSnapshot(ctx context.Context, nodeID uuid.UUID, payload []byte, requestedBy string) error {
	var compact json.RawMessage
	if err := json.Unmarshal(payload, &compact); err != nil {
		return fmt.Errorf("payload is not valid json: %w", err)
	}
	snapshot := &models.CaddySnapshot{
		NodeID:    nodeID,
		Config:    datatypes.JSON(compact),
		AppliedBy: requestedBy,
		AppliedAt: time.Now().UTC(),
	}
	return s.snapshots.Create(ctx, snapshot)
}
