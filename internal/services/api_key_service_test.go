package services_test

import (
	"context"
	"encoding/json"
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
	if err := db.AutoMigrate(&models.APIKey{}, &models.UpstreamProfile{}, &models.DomainProfile{}, &models.DiscoveryConfig{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestAPIKeyService_CreateValidateAuthorize(t *testing.T) {
	db := testAPIKeyDB(t)
	svc := services.NewAPIKeyService(repository.NewAPIKeyRepository(db), repository.NewUpstreamProfileRepository(db), repository.NewDomainProfileRepository(db))
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
	svc := services.NewAPIKeyService(repository.NewAPIKeyRepository(db), repository.NewUpstreamProfileRepository(db), repository.NewDomainProfileRepository(db))
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
	svc := services.NewAPIKeyService(repository.NewAPIKeyRepository(db), repository.NewUpstreamProfileRepository(db), repository.NewDomainProfileRepository(db))
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

func TestAPIKeyService_AuthorizeUpstreamProfile(t *testing.T) {
	db := testAPIKeyDB(t)
	discoveryRepo := repository.NewDiscoveryRepository(db)
	profileRepo := repository.NewUpstreamProfileRepository(db)
	svc := services.NewAPIKeyService(repository.NewAPIKeyRepository(db), profileRepo, repository.NewDomainProfileRepository(db))

	discoveryID := uuid.New()
	otherDiscovery := uuid.New()
	_ = discoveryRepo.Create(context.Background(), &models.DiscoveryConfig{
		ID: discoveryID, Name: "proxy", Method: models.DiscoveryMethodStaticIP, Region: "eu-west-1",
	})
	_ = discoveryRepo.Create(context.Background(), &models.DiscoveryConfig{
		ID: otherDiscovery, Name: "other", Method: models.DiscoveryMethodStaticIP, Region: "eu-west-1",
	})

	bindings, _ := json.Marshal([]models.UpstreamProfileBinding{{ConfigID: "handler", Port: 80}})
	profile := &models.UpstreamProfile{DiscoveryConfigID: discoveryID, Name: "app", Bindings: bindings}
	if err := profileRepo.Create(context.Background(), profile); err != nil {
		t.Fatalf("create profile: %v", err)
	}

	resp, err := svc.Create(context.Background(), services.CreateAPIKeyInput{
		Name:                      "proj-profile",
		Scopes:                    []string{models.APIKeyScopeRegisterUpstream},
		AllowedDiscoveryConfigIDs: []uuid.UUID{discoveryID},
		AllowedUpstreamProfileIDs: []uuid.UUID{profile.ID},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	validated, err := svc.Validate(context.Background(), resp.Secret)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if err := svc.AuthorizeUpstreamProfile(validated, profile.ID, discoveryID, models.APIKeyScopeRegisterUpstream); err != nil {
		t.Fatalf("AuthorizeUpstreamProfile allowed: %v", err)
	}
	if err := svc.AuthorizeUpstreamProfile(validated, uuid.New(), discoveryID, models.APIKeyScopeRegisterUpstream); err == nil {
		t.Fatal("expected profile forbidden")
	}
	if err := svc.AuthorizeUpstreamProfile(validated, profile.ID, otherDiscovery, models.APIKeyScopeRegisterUpstream); err == nil {
		t.Fatal("expected discovery forbidden")
	}
}

func TestAPIKeyService_AuthorizeDomainProfile(t *testing.T) {
	db := testAPIKeyDB(t)
	discoveryRepo := repository.NewDiscoveryRepository(db)
	profileRepo := repository.NewDomainProfileRepository(db)
	svc := services.NewAPIKeyService(repository.NewAPIKeyRepository(db), repository.NewUpstreamProfileRepository(db), profileRepo)

	discoveryID := uuid.New()
	otherDiscovery := uuid.New()
	_ = discoveryRepo.Create(context.Background(), &models.DiscoveryConfig{
		ID: discoveryID, Name: "proxy", Method: models.DiscoveryMethodStaticIP, Region: "eu-west-1",
	})

	bindings, _ := json.Marshal([]models.DomainProfileBinding{{ConfigID: "handler"}})
	profile := &models.DomainProfile{DiscoveryConfigID: discoveryID, Name: "tenant", Bindings: bindings}
	if err := profileRepo.Create(context.Background(), profile); err != nil {
		t.Fatalf("create profile: %v", err)
	}

	resp, err := svc.Create(context.Background(), services.CreateAPIKeyInput{
		Name:                      "proj-domain",
		Scopes:                    []string{models.APIKeyScopeRegisterDomain},
		AllowedDiscoveryConfigIDs: []uuid.UUID{discoveryID},
		AllowedDomainProfileIDs:   []uuid.UUID{profile.ID},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	validated, err := svc.Validate(context.Background(), resp.Secret)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if err := svc.AuthorizeDomainProfile(validated, profile.ID, discoveryID, models.APIKeyScopeRegisterDomain); err != nil {
		t.Fatalf("AuthorizeDomainProfile allowed: %v", err)
	}
	if err := svc.AuthorizeDomainProfile(validated, uuid.New(), discoveryID, models.APIKeyScopeRegisterDomain); err == nil {
		t.Fatal("expected domain profile forbidden")
	}
	if err := svc.AuthorizeDomainProfile(validated, profile.ID, otherDiscovery, models.APIKeyScopeRegisterDomain); err == nil {
		t.Fatal("expected discovery forbidden")
	}
}

func TestAPIKeyService_DomainProfileDiscoveryMismatchOnCreate(t *testing.T) {
	db := testAPIKeyDB(t)
	discoveryRepo := repository.NewDiscoveryRepository(db)
	profileRepo := repository.NewDomainProfileRepository(db)
	svc := services.NewAPIKeyService(repository.NewAPIKeyRepository(db), repository.NewUpstreamProfileRepository(db), profileRepo)

	discoveryID := uuid.New()
	otherDiscovery := uuid.New()
	_ = discoveryRepo.Create(context.Background(), &models.DiscoveryConfig{
		ID: discoveryID, Name: "proxy", Method: models.DiscoveryMethodStaticIP, Region: "eu-west-1",
	})
	bindings, _ := json.Marshal([]models.DomainProfileBinding{{ConfigID: "handler"}})
	profile := &models.DomainProfile{DiscoveryConfigID: discoveryID, Name: "tenant", Bindings: bindings}
	_ = profileRepo.Create(context.Background(), profile)

	_, err := svc.Create(context.Background(), services.CreateAPIKeyInput{
		Name:                      "bad-domain-key",
		Scopes:                    []string{models.APIKeyScopeRegisterDomain},
		AllowedDiscoveryConfigIDs: []uuid.UUID{otherDiscovery},
		AllowedDomainProfileIDs:   []uuid.UUID{profile.ID},
	})
	if err == nil {
		t.Fatal("expected domain profile discovery mismatch")
	}
}

func TestAPIKeyService_ProfileDiscoveryMismatchOnCreate(t *testing.T) {
	db := testAPIKeyDB(t)
	discoveryRepo := repository.NewDiscoveryRepository(db)
	profileRepo := repository.NewUpstreamProfileRepository(db)
	svc := services.NewAPIKeyService(repository.NewAPIKeyRepository(db), profileRepo, repository.NewDomainProfileRepository(db))

	discoveryID := uuid.New()
	otherDiscovery := uuid.New()
	_ = discoveryRepo.Create(context.Background(), &models.DiscoveryConfig{
		ID: discoveryID, Name: "proxy", Method: models.DiscoveryMethodStaticIP, Region: "eu-west-1",
	})
	bindings, _ := json.Marshal([]models.UpstreamProfileBinding{{ConfigID: "handler", Port: 80}})
	profile := &models.UpstreamProfile{DiscoveryConfigID: discoveryID, Name: "app", Bindings: bindings}
	_ = profileRepo.Create(context.Background(), profile)

	_, err := svc.Create(context.Background(), services.CreateAPIKeyInput{
		Name:                      "bad-key",
		Scopes:                    []string{models.APIKeyScopeRegisterUpstream},
		AllowedDiscoveryConfigIDs: []uuid.UUID{otherDiscovery},
		AllowedUpstreamProfileIDs: []uuid.UUID{profile.ID},
	})
	if err == nil {
		t.Fatal("expected profile discovery mismatch")
	}
}
