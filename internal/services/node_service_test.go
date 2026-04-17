package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

type fakeNodeRepo struct {
	getNode *models.CaddyNode
	getErr  error
}

func (f *fakeNodeRepo) List(context.Context) ([]models.CaddyNode, error) {
	return nil, errors.New("fakeNodeRepo: List not implemented")
}

func (f *fakeNodeRepo) ListPaginated(context.Context, int, int) ([]models.CaddyNode, int64, error) {
	return nil, 0, errors.New("fakeNodeRepo: ListPaginated not implemented")
}

func (f *fakeNodeRepo) GetByID(_ context.Context, _ uuid.UUID) (*models.CaddyNode, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.getNode, nil
}

func (f *fakeNodeRepo) Create(context.Context, *models.CaddyNode) error {
	return errors.New("fakeNodeRepo: Create not implemented")
}

func (f *fakeNodeRepo) Update(context.Context, *models.CaddyNode) error {
	return errors.New("fakeNodeRepo: Update not implemented")
}

func (f *fakeNodeRepo) Delete(context.Context, uuid.UUID) error {
	return errors.New("fakeNodeRepo: Delete not implemented")
}

func TestNodeService_Get_NotFound(t *testing.T) {
	svc := NewNodeService(&fakeNodeRepo{getErr: gorm.ErrRecordNotFound})
	_, err := svc.Get(context.Background(), uuid.New())
	if !errors.Is(err, models.ErrNodeNotFound) {
		t.Fatalf("Get: got %v, want models.ErrNodeNotFound", err)
	}
}

func TestNodeService_Get_RepositoryError(t *testing.T) {
	want := errors.New("db unavailable")
	svc := NewNodeService(&fakeNodeRepo{getErr: want})
	_, err := svc.Get(context.Background(), uuid.New())
	if !errors.Is(err, want) {
		t.Fatalf("Get: got %v, want %v", err, want)
	}
}

func TestNodeService_Get_Success(t *testing.T) {
	id := uuid.New()
	node := &models.CaddyNode{ID: id, Name: "edge-1", Region: "eu-west-1"}
	svc := NewNodeService(&fakeNodeRepo{getNode: node})
	got, err := svc.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got != node {
		t.Fatalf("got node %+v, want %+v", got, node)
	}
}
