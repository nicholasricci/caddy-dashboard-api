package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

type fakeDiscoveryRepo struct {
	cfg *models.DiscoveryConfig
	err error
}

func (f *fakeDiscoveryRepo) List(context.Context) ([]models.DiscoveryConfig, error) {
	return nil, errors.New("fakeDiscoveryRepo: List not implemented")
}

func (f *fakeDiscoveryRepo) GetByID(_ context.Context, _ uuid.UUID) (*models.DiscoveryConfig, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.cfg, nil
}

func (f *fakeDiscoveryRepo) Create(context.Context, *models.DiscoveryConfig) error {
	return errors.New("fakeDiscoveryRepo: Create not implemented")
}

func (f *fakeDiscoveryRepo) Update(context.Context, *models.DiscoveryConfig) error {
	return errors.New("fakeDiscoveryRepo: Update not implemented")
}

func (f *fakeDiscoveryRepo) Delete(context.Context, uuid.UUID) error {
	return errors.New("fakeDiscoveryRepo: Delete not implemented")
}

func TestDiscoveryService_Run_ConfigNotFound(t *testing.T) {
	svc := NewDiscoveryService(&fakeDiscoveryRepo{err: gorm.ErrRecordNotFound}, nil, nil, nil)
	_, err := svc.Run(context.Background(), uuid.New())
	if !errors.Is(err, ErrDiscoveryNotFound) {
		t.Fatalf("Run: got %v, want ErrDiscoveryNotFound", err)
	}
}

func TestDiscoveryService_Run_UnknownMethod(t *testing.T) {
	cfg := &models.DiscoveryConfig{
		ID:       uuid.New(),
		Name:     "test",
		Method:   "not_a_real_method",
		Region:   "eu-west-1",
		TagKey:   "k",
		TagValue: "v",
	}
	svc := NewDiscoveryService(&fakeDiscoveryRepo{cfg: cfg}, nil, nil, nil)
	_, err := svc.Run(context.Background(), cfg.ID)
	if !errors.Is(err, ErrDiscoveryUnknownMethod) {
		t.Fatalf("Run: got %v, want ErrDiscoveryUnknownMethod", err)
	}
}

func TestDiscoveryService_Run_AwsCIDRNotImplemented(t *testing.T) {
	cfg := &models.DiscoveryConfig{
		ID:       uuid.New(),
		Name:     "cidr",
		Method:   models.DiscoveryMethodAWSCIDR,
		Region:   "eu-west-1",
		TagKey:   "k",
		TagValue: "v",
	}
	svc := NewDiscoveryService(&fakeDiscoveryRepo{cfg: cfg}, nil, nil, nil)
	_, err := svc.Run(context.Background(), cfg.ID)
	if !errors.Is(err, ErrDiscoveryMethodNotImplemented) {
		t.Fatalf("Run: got %v, want ErrDiscoveryMethodNotImplemented", err)
	}
}
