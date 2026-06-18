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

func testUpstreamProfileDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.DiscoveryConfig{}, &models.UpstreamProfile{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestUpstreamProfileService_CreateAndValidateBindings(t *testing.T) {
	db := testUpstreamProfileDB(t)
	discoveryRepo := repository.NewDiscoveryRepository(db)
	profileRepo := repository.NewUpstreamProfileRepository(db)
	svc := services.NewUpstreamProfileService(profileRepo, discoveryRepo)

	discoveryID := uuid.New()
	if err := discoveryRepo.Create(context.Background(), &models.DiscoveryConfig{
		ID: discoveryID, Name: "proxy", Method: models.DiscoveryMethodStaticIP, Region: "eu-west-1",
	}); err != nil {
		t.Fatalf("create discovery: %v", err)
	}

	profile, err := svc.Create(context.Background(), services.CreateUpstreamProfileInput{
		DiscoveryConfigID: discoveryID,
		Name:              "made-in-italy-be",
		Bindings: []models.UpstreamProfileBinding{
			{ConfigID: "https-made-in-italy-api-handler", Port: 80},
			{ConfigID: "https-made-in-italy-wss-handler", Port: 8080},
		},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	bindings, err := services.ParseBindings(profile.Bindings)
	if err != nil || len(bindings) != 2 {
		t.Fatalf("bindings: %v %d", err, len(bindings))
	}

	_, err = svc.Create(context.Background(), services.CreateUpstreamProfileInput{
		DiscoveryConfigID: discoveryID,
		Name:              "made-in-italy-be",
		Bindings:          []models.UpstreamProfileBinding{{ConfigID: "x", Port: 80}},
	})
	if err == nil {
		t.Fatal("expected name taken error")
	}
}

func TestUpstreamProfileService_InvalidBindingRejected(t *testing.T) {
	db := testUpstreamProfileDB(t)
	discoveryRepo := repository.NewDiscoveryRepository(db)
	profileRepo := repository.NewUpstreamProfileRepository(db)
	svc := services.NewUpstreamProfileService(profileRepo, discoveryRepo)
	discoveryID := uuid.New()
	_ = discoveryRepo.Create(context.Background(), &models.DiscoveryConfig{
		ID: discoveryID, Name: "proxy", Method: models.DiscoveryMethodStaticIP, Region: "eu-west-1",
	})

	_, err := svc.Create(context.Background(), services.CreateUpstreamProfileInput{
		DiscoveryConfigID: discoveryID,
		Name:              "bad",
		Bindings:          []models.UpstreamProfileBinding{{ConfigID: "", Port: 80}},
	})
	if err == nil {
		t.Fatal("expected invalid binding")
	}
	_, err = svc.Create(context.Background(), services.CreateUpstreamProfileInput{
		DiscoveryConfigID: discoveryID,
		Name:              "bad2",
		Bindings:          []models.UpstreamProfileBinding{{ConfigID: "x", Port: 0}},
	})
	if err == nil {
		t.Fatal("expected invalid port")
	}
}

func TestParseBindings_EmptyRejected(t *testing.T) {
	_, err := services.ParseBindings(json.RawMessage(`[]`))
	if err == nil {
		t.Fatal("expected empty bindings error")
	}
}
