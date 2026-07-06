package repository

import (
	"context"
	"testing"

	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository/testutil"
)

func TestAuditRepository_ListPaginated_ActorExactMatch(t *testing.T) {
	db := testutil.NewTestDB(t)
	if err := db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewAuditRepository(db)
	ctx := context.Background()

	seedAuditActors(t, repo, ctx, "admin", "administrator", "scheduler")

	items, total, err := repo.ListPaginated(ctx, models.AuditListFilter{Actor: "admin"}, 10, 0)
	if err != nil {
		t.Fatalf("ListPaginated: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("exact match: total=%d items=%d, want 1", total, len(items))
	}
	if items[0].Actor != "admin" {
		t.Fatalf("exact match actor=%q, want admin", items[0].Actor)
	}
}

func TestAuditRepository_ListPaginated_ActorPrefixWildcard(t *testing.T) {
	db := testutil.NewTestDB(t)
	if err := db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewAuditRepository(db)
	ctx := context.Background()

	seedAuditActors(t, repo, ctx, "admin", "administrator", "scheduler")

	items, total, err := repo.ListPaginated(ctx, models.AuditListFilter{Actor: "adm*"}, 10, 0)
	if err != nil {
		t.Fatalf("ListPaginated: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("prefix wildcard: total=%d items=%d, want 2", total, len(items))
	}
}

func TestAuditRepository_ListPaginated_ActorContainsWildcard(t *testing.T) {
	db := testutil.NewTestDB(t)
	if err := db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewAuditRepository(db)
	ctx := context.Background()

	seedAuditActors(t, repo, ctx, "admin", "job-scheduler", "scheduler-bot")

	items, total, err := repo.ListPaginated(ctx, models.AuditListFilter{Actor: "*scheduler*"}, 10, 0)
	if err != nil {
		t.Fatalf("ListPaginated: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("contains wildcard: total=%d items=%d, want 2", total, len(items))
	}
}

func TestAuditRepository_ListPaginated_ActorLiteralPercentAndUnderscore(t *testing.T) {
	db := testutil.NewTestDB(t)
	if err := db.AutoMigrate(&models.AuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := NewAuditRepository(db)
	ctx := context.Background()

	seedAuditActors(t, repo, ctx, "svc_%_bot", "svcXbot", "other")

	items, total, err := repo.ListPaginated(ctx, models.AuditListFilter{Actor: "svc_%_bot"}, 10, 0)
	if err != nil {
		t.Fatalf("ListPaginated: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].Actor != "svc_%_bot" {
		t.Fatalf("literal %% and _: total=%d items=%d", total, len(items))
	}
}

func TestActorPatternToLike(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"adm*", "adm%"},
		{"*scheduler*", "%scheduler%"},
		{"svc_%_bot", `svc\_\%\_bot`},
		{`a\b*c`, `a\\b%c`},
	}
	for _, tc := range tests {
		got := actorPatternToLike(tc.in)
		if got != tc.want {
			t.Fatalf("actorPatternToLike(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func seedAuditActors(t *testing.T, repo *AuditRepository, ctx context.Context, actors ...string) {
	t.Helper()
	for _, actor := range actors {
		if err := repo.Create(ctx, &models.AuditLog{
			Actor:    actor,
			Action:   "sync",
			Resource: "node",
		}); err != nil {
			t.Fatalf("create audit log for %q: %v", actor, err)
		}
	}
}
