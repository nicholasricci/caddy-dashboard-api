package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository/testutil"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type backfillSeedData struct {
	groupConfigID            uuid.UUID
	nodeConfigID             uuid.UUID
	groupDeletedConfigID     uuid.UUID
	groupNodeID              uuid.UUID
	nodeScopeNodeID          uuid.UUID
	groupDeletedConfigNodeID uuid.UUID
	deletedNodeID            uuid.UUID
	groupSnapshotID          uuid.UUID
	nodeScopeSnapshotID      uuid.UUID
	groupDeletedCfgSnapID    uuid.UUID
	deletedNodeSnapshotID    uuid.UUID
	alreadyFilledSnapshotID  uuid.UUID
}

func TestBackfill_UpdatesOnlyGroupScopedNodeScopedSnapshots(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSnapshotRepository(db)
	ctx := context.Background()
	seed := seedBackfillScenario(t, db)

	rows, err := repo.BackfillDiscoveryConfigIDs(ctx)
	if err != nil {
		t.Fatalf("BackfillDiscoveryConfigIDs: %v", err)
	}
	if rows != 1 {
		t.Fatalf("rows=%d, want 1", rows)
	}

	groupSnap := mustLoadSnapshot(t, db, seed.groupSnapshotID)
	if groupSnap.DiscoveryConfigID == nil || *groupSnap.DiscoveryConfigID != seed.groupConfigID {
		t.Fatalf("group snapshot discovery_config_id=%v, want %s", groupSnap.DiscoveryConfigID, seed.groupConfigID)
	}

	nodeScopeSnap := mustLoadSnapshot(t, db, seed.nodeScopeSnapshotID)
	if nodeScopeSnap.DiscoveryConfigID != nil {
		t.Fatalf("node-scope snapshot discovery_config_id=%v, want nil", nodeScopeSnap.DiscoveryConfigID)
	}

	groupDeletedCfgSnap := mustLoadSnapshot(t, db, seed.groupDeletedCfgSnapID)
	if groupDeletedCfgSnap.DiscoveryConfigID != nil {
		t.Fatalf("deleted-config snapshot discovery_config_id=%v, want nil", groupDeletedCfgSnap.DiscoveryConfigID)
	}

	deletedNodeSnap := mustLoadSnapshot(t, db, seed.deletedNodeSnapshotID)
	if deletedNodeSnap.DiscoveryConfigID != nil {
		t.Fatalf("deleted-node snapshot discovery_config_id=%v, want nil", deletedNodeSnap.DiscoveryConfigID)
	}

	alreadyFilled := mustLoadSnapshot(t, db, seed.alreadyFilledSnapshotID)
	if alreadyFilled.DiscoveryConfigID == nil || *alreadyFilled.DiscoveryConfigID != seed.groupConfigID {
		t.Fatalf("already-filled discovery_config_id=%v, want %s", alreadyFilled.DiscoveryConfigID, seed.groupConfigID)
	}
}

func TestBackfill_Idempotent(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSnapshotRepository(db)
	ctx := context.Background()
	_ = seedBackfillScenario(t, db)

	rows, err := repo.BackfillDiscoveryConfigIDs(ctx)
	if err != nil {
		t.Fatalf("first backfill: %v", err)
	}
	if rows != 1 {
		t.Fatalf("first rows=%d, want 1", rows)
	}

	rows, err = repo.BackfillDiscoveryConfigIDs(ctx)
	if err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if rows != 0 {
		t.Fatalf("second rows=%d, want 0", rows)
	}
}

func TestBackfill_SkipsNodeScopeConfigs(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSnapshotRepository(db)
	ctx := context.Background()
	seed := seedBackfillScenario(t, db)

	if _, err := repo.BackfillDiscoveryConfigIDs(ctx); err != nil {
		t.Fatalf("BackfillDiscoveryConfigIDs: %v", err)
	}

	nodeScopeSnap := mustLoadSnapshot(t, db, seed.nodeScopeSnapshotID)
	if nodeScopeSnap.DiscoveryConfigID != nil {
		t.Fatalf("node-scope snapshot discovery_config_id=%v, want nil", nodeScopeSnap.DiscoveryConfigID)
	}
}

func TestBackfill_SkipsSoftDeletedNodeAndConfig(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSnapshotRepository(db)
	ctx := context.Background()
	seed := seedBackfillScenario(t, db)

	if _, err := repo.BackfillDiscoveryConfigIDs(ctx); err != nil {
		t.Fatalf("BackfillDiscoveryConfigIDs: %v", err)
	}

	groupDeletedCfgSnap := mustLoadSnapshot(t, db, seed.groupDeletedCfgSnapID)
	if groupDeletedCfgSnap.DiscoveryConfigID != nil {
		t.Fatalf("deleted-config snapshot discovery_config_id=%v, want nil", groupDeletedCfgSnap.DiscoveryConfigID)
	}
	deletedNodeSnap := mustLoadSnapshot(t, db, seed.deletedNodeSnapshotID)
	if deletedNodeSnap.DiscoveryConfigID != nil {
		t.Fatalf("deleted-node snapshot discovery_config_id=%v, want nil", deletedNodeSnap.DiscoveryConfigID)
	}
}

func TestBackfill_NoOpOnEmptyDB(t *testing.T) {
	db := testutil.NewTestDB(t)
	repo := NewSnapshotRepository(db)

	rows, err := repo.BackfillDiscoveryConfigIDs(context.Background())
	if err != nil {
		t.Fatalf("BackfillDiscoveryConfigIDs: %v", err)
	}
	if rows != 0 {
		t.Fatalf("rows=%d, want 0", rows)
	}
}

func seedBackfillScenario(t *testing.T, db *gorm.DB) backfillSeedData {
	t.Helper()

	groupCfg := models.DiscoveryConfig{
		ID:            uuid.New(),
		Name:          "group",
		Method:        models.DiscoveryMethodStaticIP,
		Region:        "eu-west-1",
		TagKey:        "k",
		TagValue:      "v",
		SnapshotScope: models.SnapshotScopeGroup,
		Enabled:       true,
	}
	nodeCfg := models.DiscoveryConfig{
		ID:            uuid.New(),
		Name:          "node",
		Method:        models.DiscoveryMethodStaticIP,
		Region:        "eu-west-1",
		TagKey:        "k",
		TagValue:      "v",
		SnapshotScope: models.SnapshotScopeNode,
		Enabled:       true,
	}
	groupDeletedCfg := models.DiscoveryConfig{
		ID:            uuid.New(),
		Name:          "group-deleted",
		Method:        models.DiscoveryMethodStaticIP,
		Region:        "eu-west-1",
		TagKey:        "k",
		TagValue:      "v",
		SnapshotScope: models.SnapshotScopeGroup,
		Enabled:       true,
	}
	mustCreate(t, db, &groupCfg)
	mustCreate(t, db, &nodeCfg)
	mustCreate(t, db, &groupDeletedCfg)

	groupNode := models.CaddyNode{
		ID:                uuid.New(),
		Name:              "group-node",
		PrivateIP:         strPtr("10.0.0.10"),
		Region:            "eu-west-1",
		Status:            "online",
		DiscoveryConfigID: &groupCfg.ID,
		SSMEnabled:        true,
	}
	nodeScopeNode := models.CaddyNode{
		ID:                uuid.New(),
		Name:              "node-scope-node",
		PrivateIP:         strPtr("10.0.0.11"),
		Region:            "eu-west-1",
		Status:            "online",
		DiscoveryConfigID: &nodeCfg.ID,
		SSMEnabled:        true,
	}
	groupDeletedCfgNode := models.CaddyNode{
		ID:                uuid.New(),
		Name:              "group-deleted-cfg-node",
		PrivateIP:         strPtr("10.0.0.12"),
		Region:            "eu-west-1",
		Status:            "online",
		DiscoveryConfigID: &groupDeletedCfg.ID,
		SSMEnabled:        true,
	}
	deletedNode := models.CaddyNode{
		ID:                uuid.New(),
		Name:              "deleted-node",
		PrivateIP:         strPtr("10.0.0.13"),
		Region:            "eu-west-1",
		Status:            "online",
		DiscoveryConfigID: &groupCfg.ID,
		SSMEnabled:        true,
	}
	mustCreate(t, db, &groupNode)
	mustCreate(t, db, &nodeScopeNode)
	mustCreate(t, db, &groupDeletedCfgNode)
	mustCreate(t, db, &deletedNode)

	// Soft-delete one config and one node: they must be ignored by backfill.
	if err := db.Delete(&groupDeletedCfg).Error; err != nil {
		t.Fatalf("delete groupDeletedCfg: %v", err)
	}
	if err := db.Delete(&deletedNode).Error; err != nil {
		t.Fatalf("delete deletedNode: %v", err)
	}

	now := time.Now().UTC()
	groupSnap := models.CaddySnapshot{
		ID:        uuid.New(),
		NodeID:    &groupNode.ID,
		Config:    datatypes.JSON([]byte(`{"apps":{}}`)),
		AppliedBy: "tester",
		AppliedAt: now,
	}
	nodeScopeSnap := models.CaddySnapshot{
		ID:        uuid.New(),
		NodeID:    &nodeScopeNode.ID,
		Config:    datatypes.JSON([]byte(`{"apps":{}}`)),
		AppliedBy: "tester",
		AppliedAt: now.Add(time.Second),
	}
	groupDeletedCfgSnap := models.CaddySnapshot{
		ID:        uuid.New(),
		NodeID:    &groupDeletedCfgNode.ID,
		Config:    datatypes.JSON([]byte(`{"apps":{}}`)),
		AppliedBy: "tester",
		AppliedAt: now.Add(2 * time.Second),
	}
	deletedNodeSnap := models.CaddySnapshot{
		ID:        uuid.New(),
		NodeID:    &deletedNode.ID,
		Config:    datatypes.JSON([]byte(`{"apps":{}}`)),
		AppliedBy: "tester",
		AppliedAt: now.Add(3 * time.Second),
	}
	alreadyFilled := models.CaddySnapshot{
		ID:                uuid.New(),
		NodeID:            &groupNode.ID,
		DiscoveryConfigID: &groupCfg.ID,
		Config:            datatypes.JSON([]byte(`{"apps":{}}`)),
		AppliedBy:         "tester",
		AppliedAt:         now.Add(4 * time.Second),
	}
	mustCreate(t, db, &groupSnap)
	mustCreate(t, db, &nodeScopeSnap)
	mustCreate(t, db, &groupDeletedCfgSnap)
	mustCreate(t, db, &deletedNodeSnap)
	mustCreate(t, db, &alreadyFilled)

	return backfillSeedData{
		groupConfigID:            groupCfg.ID,
		nodeConfigID:             nodeCfg.ID,
		groupDeletedConfigID:     groupDeletedCfg.ID,
		groupNodeID:              groupNode.ID,
		nodeScopeNodeID:          nodeScopeNode.ID,
		groupDeletedConfigNodeID: groupDeletedCfgNode.ID,
		deletedNodeID:            deletedNode.ID,
		groupSnapshotID:          groupSnap.ID,
		nodeScopeSnapshotID:      nodeScopeSnap.ID,
		groupDeletedCfgSnapID:    groupDeletedCfgSnap.ID,
		deletedNodeSnapshotID:    deletedNodeSnap.ID,
		alreadyFilledSnapshotID:  alreadyFilled.ID,
	}
}

func mustCreate(t *testing.T, db *gorm.DB, model any) {
	t.Helper()
	if err := db.Create(model).Error; err != nil {
		t.Fatalf("create %T: %v", model, err)
	}
}

func mustLoadSnapshot(t *testing.T, db *gorm.DB, id uuid.UUID) models.CaddySnapshot {
	t.Helper()
	var s models.CaddySnapshot
	if err := db.First(&s, "id = ?", id).Error; err != nil {
		t.Fatalf("load snapshot %s: %v", id, err)
	}
	return s
}
