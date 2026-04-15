package caddy

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

type fakeNodeLoader struct {
	node *models.CaddyNode
	err  error
}

func (f *fakeNodeLoader) GetByID(_ context.Context, _ uuid.UUID) (*models.CaddyNode, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.node, nil
}

func TestService_GetLiveConfig_NodeNotFound(t *testing.T) {
	svc := NewService(&fakeNodeLoader{err: gorm.ErrRecordNotFound}, nil, nil)
	_, err := svc.GetLiveConfig(context.Background(), uuid.New())
	if !errors.Is(err, models.ErrNodeNotFound) {
		t.Fatalf("GetLiveConfig: got %v, want models.ErrNodeNotFound", err)
	}
}

func TestService_GetLiveConfig_NoInstanceID(t *testing.T) {
	node := &models.CaddyNode{
		ID:         uuid.New(),
		Name:       "n",
		Region:     "eu-west-1",
		InstanceID: nil,
	}
	svc := NewService(&fakeNodeLoader{node: node}, nil, nil)
	_, err := svc.GetLiveConfig(context.Background(), node.ID)
	if !errors.Is(err, ErrNodeNoInstanceID) {
		t.Fatalf("GetLiveConfig: got %v, want ErrNodeNoInstanceID", err)
	}
}
