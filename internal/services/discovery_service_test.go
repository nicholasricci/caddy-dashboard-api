package services

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"gorm.io/gorm"
)

type fakeDiscoveryNodeWriter struct {
	upserted []*models.CaddyNode
	err      error
}

func (f *fakeDiscoveryNodeWriter) UpsertByInstanceOrIP(_ context.Context, node *models.CaddyNode) error {
	if f.err != nil {
		return f.err
	}
	copyNode := *node
	f.upserted = append(f.upserted, &copyNode)
	return nil
}

type fakeDiscoveryRepo struct {
	cfg *models.DiscoveryConfig
	err error
}

func (f *fakeDiscoveryRepo) List(context.Context) ([]models.DiscoveryConfig, error) {
	return nil, errors.New("fakeDiscoveryRepo: List not implemented")
}

func (f *fakeDiscoveryRepo) ListPaginated(context.Context, int, int) ([]models.DiscoveryConfig, int64, error) {
	return nil, 0, errors.New("fakeDiscoveryRepo: ListPaginated not implemented")
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

func TestDiscoveryService_Run_Errors(t *testing.T) {
	tests := []struct {
		name    string
		repo    *fakeDiscoveryRepo
		wantErr error
	}{
		{
			name:    "config not found",
			repo:    &fakeDiscoveryRepo{err: gorm.ErrRecordNotFound},
			wantErr: ErrDiscoveryNotFound,
		},
		{
			name: "unknown method",
			repo: &fakeDiscoveryRepo{cfg: &models.DiscoveryConfig{
				ID:       uuid.New(),
				Name:     "test",
				Method:   "not_a_real_method",
				Region:   "eu-west-1",
				TagKey:   "k",
				TagValue: "v",
			}},
			wantErr: ErrDiscoveryUnknownMethod,
		},
		{
			name: "aws cidr not implemented",
			repo: &fakeDiscoveryRepo{cfg: &models.DiscoveryConfig{
				ID:       uuid.New(),
				Name:     "cidr",
				Method:   models.DiscoveryMethodAWSCIDR,
				Region:   "eu-west-1",
				TagKey:   "k",
				TagValue: "v",
			}},
			wantErr: ErrDiscoveryMethodNotImplemented,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			svc := NewDiscoveryService(tc.repo, nil, nil, nil)
			_, err := svc.Run(context.Background(), uuid.New())
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Run: got %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestDiscoveryService_Create_DefaultSnapshotScope(t *testing.T) {
	repo := &fakeDiscoveryRepo{}
	svc := NewDiscoveryService(repo, nil, nil, nil)
	cfg := &models.DiscoveryConfig{
		ID:     uuid.New(),
		Name:   "d1",
		Method: models.DiscoveryMethodAWSTag,
		Region: "eu-west-1",
	}
	if err := svc.Create(context.Background(), cfg); err == nil {
		t.Fatal("Create: expected fake repo error")
	}
	if cfg.SnapshotScope != models.SnapshotScopeNode {
		t.Fatalf("Create: snapshot_scope=%q, want %q", cfg.SnapshotScope, models.SnapshotScopeNode)
	}
}

func TestDiscoveryService_Run_StaticIP_AssignsDiscoveryConfigID(t *testing.T) {
	cfgID := uuid.New()
	params, _ := json.Marshal(map[string]any{
		"endpoints": []map[string]string{
			{"private_ip": "10.0.0.10", "name": "node-a"},
		},
	})
	cfg := &models.DiscoveryConfig{
		ID:         cfgID,
		Name:       "static",
		Method:     models.DiscoveryMethodStaticIP,
		Region:     "eu-west-1",
		Parameters: params,
	}
	writer := &fakeDiscoveryNodeWriter{}
	svc := NewDiscoveryService(&fakeDiscoveryRepo{cfg: cfg}, writer, nil, nil)

	n, err := svc.Run(context.Background(), cfgID)
	if err != nil {
		t.Fatalf("Run: unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("Run: discovered=%d, want 1", n)
	}
	if len(writer.upserted) != 1 {
		t.Fatalf("upserts=%d, want 1", len(writer.upserted))
	}
	got := writer.upserted[0].DiscoveryConfigID
	if got == nil || *got != cfgID {
		t.Fatalf("DiscoveryConfigID=%v, want %s", got, cfgID)
	}
}
