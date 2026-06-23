package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestParseAuditListFilter_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/audit", nil)

	filter, err := parseAuditListFilter(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filter.Action != "" || filter.Resource != "" || filter.Actor != "" || filter.ResourceID != "" {
		t.Fatalf("expected empty filter, got %+v", filter)
	}
	if filter.From != nil || filter.To != nil {
		t.Fatalf("expected no date bounds, got %+v", filter)
	}
}

func TestParseAuditListFilter_ValidEnums(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/audit?action=sync&resource=node", nil)

	filter, err := parseAuditListFilter(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filter.Action != "sync" || filter.Resource != "node" {
		t.Fatalf("unexpected filter: %+v", filter)
	}
}

func TestParseAuditListFilter_InvalidAction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/audit?action=foobar", nil)

	_, err := parseAuditListFilter(c)
	if err == nil {
		t.Fatal("expected error for invalid action")
	}
}

func TestParseAuditListFilter_InvalidResource(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/audit?resource=unknown", nil)

	_, err := parseAuditListFilter(c)
	if err == nil {
		t.Fatal("expected error for invalid resource")
	}
}

func TestParseAuditListFilter_ValidDates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/audit?from=2024-01-01T00:00:00Z&to=2024-12-31T23:59:59Z", nil)

	filter, err := parseAuditListFilter(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filter.From == nil || filter.To == nil {
		t.Fatal("expected date bounds")
	}
}

func TestParseAuditListFilter_InvalidFrom(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/audit?from=not-a-date", nil)

	_, err := parseAuditListFilter(c)
	if err == nil {
		t.Fatal("expected error for invalid from")
	}
}

func TestParseAuditListFilter_FromAfterTo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/audit?from=2024-12-31T23:59:59Z&to=2024-01-01T00:00:00Z", nil)

	_, err := parseAuditListFilter(c)
	if err == nil {
		t.Fatal("expected error when from is after to")
	}
}

func TestParseAuditListFilter_TrimsActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/audit?actor=%20admin%20", nil)

	filter, err := parseAuditListFilter(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filter.Actor != "admin" {
		t.Fatalf("expected trimmed actor, got %q", filter.Actor)
	}
}

func TestAuditHandler_ListTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/audit/types", nil)

	h := NewAuditHandler(nil, nil)
	h.ListTypes(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestParseAuditListFilter_DateBoundaryEqual(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ts := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/audit?from="+ts+"&to="+ts, nil)

	_, err := parseAuditListFilter(c)
	if err != nil {
		t.Fatalf("equal from/to should be valid: %v", err)
	}
}
