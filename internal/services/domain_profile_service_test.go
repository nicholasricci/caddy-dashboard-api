package services_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/nicholasricci/caddy-dashboard/internal/models"
	"github.com/nicholasricci/caddy-dashboard/internal/repository"
	"github.com/nicholasricci/caddy-dashboard/internal/services"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testDomainProfileDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.DiscoveryConfig{}, &models.DomainProfile{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestDomainProfileService_CreateAndValidateBindings(t *testing.T) {
	db := testDomainProfileDB(t)
	discoveryRepo := repository.NewDiscoveryRepository(db)
	profileRepo := repository.NewDomainProfileRepository(db)
	svc := services.NewDomainProfileService(profileRepo, discoveryRepo)

	discoveryID := uuid.New()
	if err := discoveryRepo.Create(context.Background(), &models.DiscoveryConfig{
		ID: discoveryID, Name: "proxy", Method: models.DiscoveryMethodStaticIP, Region: "eu-west-1",
	}); err != nil {
		t.Fatalf("create discovery: %v", err)
	}

	profile, err := svc.Create(context.Background(), services.CreateDomainProfileInput{
		DiscoveryConfigID: discoveryID,
		Name:              "tenant-routes",
		Bindings: []models.DomainProfileBinding{
			{ConfigID: "https-api-handler", MatchIndexes: []int{0}},
			{ConfigID: "https-wss-handler"},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	bindings, err := services.ParseDomainBindings(profile.Bindings)
	if err != nil || len(bindings) != 2 {
		t.Fatalf("bindings: %v %d", err, len(bindings))
	}
	if len(bindings[1].MatchIndexes) != 1 || bindings[1].MatchIndexes[0] != 0 {
		t.Fatalf("default match index: %+v", bindings[1])
	}

	_, err = svc.Create(context.Background(), services.CreateDomainProfileInput{
		DiscoveryConfigID: discoveryID,
		Name:              "tenant-routes",
		Bindings:          []models.DomainProfileBinding{{ConfigID: "x"}},
	})
	if err == nil {
		t.Fatal("expected name taken error")
	}
}

func TestDomainProfileService_InvalidBindingRejected(t *testing.T) {
	db := testDomainProfileDB(t)
	discoveryRepo := repository.NewDiscoveryRepository(db)
	profileRepo := repository.NewDomainProfileRepository(db)
	svc := services.NewDomainProfileService(profileRepo, discoveryRepo)
	discoveryID := uuid.New()
	_ = discoveryRepo.Create(context.Background(), &models.DiscoveryConfig{
		ID: discoveryID, Name: "proxy", Method: models.DiscoveryMethodStaticIP, Region: "eu-west-1",
	})

	_, err := svc.Create(context.Background(), services.CreateDomainProfileInput{
		DiscoveryConfigID: discoveryID,
		Name:              "bad",
		Bindings:          []models.DomainProfileBinding{{ConfigID: ""}},
	})
	if err == nil {
		t.Fatal("expected invalid binding")
	}
	_, err = svc.Create(context.Background(), services.CreateDomainProfileInput{
		DiscoveryConfigID: discoveryID,
		Name:              "bad2",
		Bindings:          []models.DomainProfileBinding{{ConfigID: "x", MatchIndexes: []int{-1}}},
	})
	if err == nil {
		t.Fatal("expected invalid match index")
	}
}

func TestParseDomainBindings_EmptyRejected(t *testing.T) {
	_, err := services.ParseDomainBindings(json.RawMessage(`[]`))
	if err == nil {
		t.Fatal("expected empty bindings error")
	}
}
