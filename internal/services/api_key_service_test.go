package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testAPIKeyDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.APIKey{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestAPIKeyService_CreateValidateAuthorize(t *testing.T) {
	db := testAPIKeyDB(t)
	svc := services.NewAPIKeyService(repository.NewAPIKeyRepository(db))
	discoveryID := uuid.New()

	resp, err := svc.Create(context.Background(), services.CreateAPIKeyInput{
		Name:                      "proj-a",
		Scopes:                    []string{models.APIKeyScopeRegisterUpstream},
		AllowedDiscoveryConfigIDs: []uuid.UUID{discoveryID},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	validated, err := svc.Validate(context.Background(), resp.Secret)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if validated.Name != "proj-a" {
		t.Fatalf("name=%q", validated.Name)
	}
	if err := svc.AuthorizeDiscovery(validated, discoveryID, models.APIKeyScopeRegisterUpstream); err != nil {
		t.Fatalf("AuthorizeDiscovery allowed: %v", err)
	}
	if err := svc.AuthorizeDiscovery(validated, uuid.New(), models.APIKeyScopeRegisterUpstream); err == nil {
		t.Fatal("expected forbidden for other discovery id")
	}
}

func TestAPIKeyService_RevokedKeyRejected(t *testing.T) {
	db := testAPIKeyDB(t)
	svc := services.NewAPIKeyService(repository.NewAPIKeyRepository(db))
	discoveryID := uuid.New()

	resp, err := svc.Create(context.Background(), services.CreateAPIKeyInput{
		Name:                      "proj-b",
		Scopes:                    []string{models.APIKeyScopeRegisterUpstream},
		AllowedDiscoveryConfigIDs: []uuid.UUID{discoveryID},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.Revoke(context.Background(), resp.APIKey.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if _, err := svc.Validate(context.Background(), resp.Secret); err == nil {
		t.Fatal("expected revoked error")
	} else if err != services.ErrAPIKeyRevoked && err.Error() != services.ErrAPIKeyRevoked.Error() {
		// errors.Is works in production; direct compare for simplicity
		if err.Error() != "api key revoked" {
			t.Fatalf("err=%v", err)
		}
	}
}

func TestAPIKeyService_ExpiredKeyRejected(t *testing.T) {
	db := testAPIKeyDB(t)
	svc := services.NewAPIKeyService(repository.NewAPIKeyRepository(db))
	past := time.Now().UTC().Add(-time.Hour)
	resp, err := svc.Create(context.Background(), services.CreateAPIKeyInput{
		Name:                      "expired",
		Scopes:                    []string{models.APIKeyScopeRegisterUpstream},
		AllowedDiscoveryConfigIDs: []uuid.UUID{uuid.New()},
		ExpiresAt:                 &past,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := svc.Validate(context.Background(), resp.Secret); err == nil {
		t.Fatal("expected expired error")
	}
}
