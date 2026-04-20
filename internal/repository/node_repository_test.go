package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository/testutil"
)

func TestUpsert_FirstInsert_NoExisting(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewNodeRepository(db)
	ctx := context.Background()

	cfgID := uuid.New()
	node := &models.CaddyNode{
		Name:              "node-a",
		InstanceID:        strPtr("i-abc"),
		PrivateIP:         strPtr("10.0.0.10"),
		Region:            "eu-west-1",
		Status:            "online",
		LastSeenAt:        timePtr(time.Now().UTC()),
		DiscoveryConfigID: &cfgID,
	}

	if err := repo.UpsertByInstanceOrIP(ctx, node); err != nil {
		t.Fatalf("UpsertByInstanceOrIP: %v", err)
	}
	if node.ID == uuid.Nil {
		t.Fatal("expected generated node ID")
	}
	got, err := repo.GetByID(ctx, node.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.DiscoveryConfigID == nil || *got.DiscoveryConfigID != cfgID {
		t.Fatalf("DiscoveryConfigID=%v, want %s", got.DiscoveryConfigID, cfgID)
	}
}

func TestUpsert_MatchByInstanceID_UpdatesExisting(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewNodeRepository(db)
	ctx := context.Background()

	initial := &models.CaddyNode{
		Name:       "old-name",
		InstanceID: strPtr("i-match"),
		PrivateIP:  strPtr("10.0.0.11"),
		Region:     "eu-west-1",
		Status:     "unknown",
		LastSeenAt: timePtr(time.Now().Add(-1 * time.Hour).UTC()),
	}
	if err := repo.Create(ctx, initial); err != nil {
		t.Fatalf("Create: %v", err)
	}

	incoming := &models.CaddyNode{
		Name:       "new-name",
		InstanceID: strPtr("i-match"),
		PrivateIP:  strPtr("10.0.0.99"),
		Region:     "eu-west-1",
		Status:     "online",
		LastSeenAt: timePtr(time.Now().UTC()),
	}
	if err := repo.UpsertByInstanceOrIP(ctx, incoming); err != nil {
		t.Fatalf("UpsertByInstanceOrIP: %v", err)
	}

	got, err := repo.GetByID(ctx, initial.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "new-name" {
		t.Fatalf("Name=%q, want %q", got.Name, "new-name")
	}
	if got.PrivateIP == nil || *got.PrivateIP != "10.0.0.99" {
		t.Fatalf("PrivateIP=%v, want 10.0.0.99", got.PrivateIP)
	}
	if got.ID != initial.ID {
		t.Fatalf("ID changed: got=%s want=%s", got.ID, initial.ID)
	}
}

func TestUpsert_MatchByPrivateIPAndRegion_UpdatesExisting(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewNodeRepository(db)
	ctx := context.Background()

	initial := &models.CaddyNode{
		Name:      "old",
		PrivateIP: strPtr("10.0.0.20"),
		Region:    "eu-west-1",
		Status:    "unknown",
	}
	if err := repo.Create(ctx, initial); err != nil {
		t.Fatalf("Create: %v", err)
	}

	incoming := &models.CaddyNode{
		Name:      "new",
		PrivateIP: strPtr("10.0.0.20"),
		Region:    "eu-west-1",
		Status:    "online",
	}
	if err := repo.UpsertByInstanceOrIP(ctx, incoming); err != nil {
		t.Fatalf("UpsertByInstanceOrIP: %v", err)
	}

	got, err := repo.GetByID(ctx, initial.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Name != "new" {
		t.Fatalf("Name=%q, want %q", got.Name, "new")
	}
}

func TestUpsert_KeepFirst_DoesNotOverwriteDiscoveryConfigID(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewNodeRepository(db)
	ctx := context.Background()

	firstCfg := uuid.New()
	secondCfg := uuid.New()
	instanceID := "i-keep-first"

	initial := &models.CaddyNode{
		Name:              "node-keep",
		InstanceID:        &instanceID,
		Region:            "eu-west-1",
		Status:            "online",
		DiscoveryConfigID: &firstCfg,
	}
	if err := repo.Create(ctx, initial); err != nil {
		t.Fatalf("Create: %v", err)
	}

	incoming := &models.CaddyNode{
		Name:              "node-keep",
		InstanceID:        &instanceID,
		Region:            "eu-west-1",
		Status:            "online",
		DiscoveryConfigID: &secondCfg,
	}
	if err := repo.UpsertByInstanceOrIP(ctx, incoming); err != nil {
		t.Fatalf("UpsertByInstanceOrIP: %v", err)
	}

	got, err := repo.GetByID(ctx, initial.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.DiscoveryConfigID == nil || *got.DiscoveryConfigID != firstCfg {
		t.Fatalf("DiscoveryConfigID=%v, want %s", got.DiscoveryConfigID, firstCfg)
	}
}

func TestUpsert_FillsDiscoveryConfigIDWhenExistingIsNil(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewNodeRepository(db)
	ctx := context.Background()

	cfgID := uuid.New()
	instanceID := "i-fill-cfg"

	initial := &models.CaddyNode{
		Name:       "node-fill",
		InstanceID: &instanceID,
		Region:     "eu-west-1",
		Status:     "online",
	}
	if err := repo.Create(ctx, initial); err != nil {
		t.Fatalf("Create: %v", err)
	}

	incoming := &models.CaddyNode{
		Name:              "node-fill",
		InstanceID:        &instanceID,
		Region:            "eu-west-1",
		Status:            "online",
		DiscoveryConfigID: &cfgID,
	}
	if err := repo.UpsertByInstanceOrIP(ctx, incoming); err != nil {
		t.Fatalf("UpsertByInstanceOrIP: %v", err)
	}

	got, err := repo.GetByID(ctx, initial.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.DiscoveryConfigID == nil || *got.DiscoveryConfigID != cfgID {
		t.Fatalf("DiscoveryConfigID=%v, want %s", got.DiscoveryConfigID, cfgID)
	}
}

func TestUpsert_DoesNotOverwriteStatusWithEmpty(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewNodeRepository(db)
	ctx := context.Background()

	instanceID := "i-status"
	initial := &models.CaddyNode{
		Name:       "node-status",
		InstanceID: &instanceID,
		Region:     "eu-west-1",
		Status:     "online",
	}
	if err := repo.Create(ctx, initial); err != nil {
		t.Fatalf("Create: %v", err)
	}

	incoming := &models.CaddyNode{
		Name:       "node-status",
		InstanceID: &instanceID,
		Region:     "eu-west-1",
		Status:     "",
	}
	if err := repo.UpsertByInstanceOrIP(ctx, incoming); err != nil {
		t.Fatalf("UpsertByInstanceOrIP: %v", err)
	}

	got, err := repo.GetByID(ctx, initial.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Status != "online" {
		t.Fatalf("Status=%q, want %q", got.Status, "online")
	}
}

func TestUpsert_DoesNotOverwriteLastSeenAtWithNil(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewNodeRepository(db)
	ctx := context.Background()

	instanceID := "i-last-seen"
	lastSeen := time.Now().Add(-5 * time.Minute).UTC()
	initial := &models.CaddyNode{
		Name:       "node-last-seen",
		InstanceID: &instanceID,
		Region:     "eu-west-1",
		Status:     "online",
		LastSeenAt: &lastSeen,
	}
	if err := repo.Create(ctx, initial); err != nil {
		t.Fatalf("Create: %v", err)
	}

	incoming := &models.CaddyNode{
		Name:       "node-last-seen",
		InstanceID: &instanceID,
		Region:     "eu-west-1",
		Status:     "online",
		LastSeenAt: nil,
	}
	if err := repo.UpsertByInstanceOrIP(ctx, incoming); err != nil {
		t.Fatalf("UpsertByInstanceOrIP: %v", err)
	}

	got, err := repo.GetByID(ctx, initial.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.LastSeenAt == nil || !got.LastSeenAt.Equal(lastSeen) {
		t.Fatalf("LastSeenAt=%v, want %v", got.LastSeenAt, lastSeen)
	}
}

func strPtr(v string) *string {
	return &v
}

func timePtr(v time.Time) *time.Time {
	return &v
}
