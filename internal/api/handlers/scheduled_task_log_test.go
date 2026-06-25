package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestParseScheduledTaskLogListFilter_Empty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/scheduled-tasks/abc/logs", nil)

	filter, err := parseScheduledTaskLogListFilter(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filter.Status != "" {
		t.Fatalf("expected empty status, got %q", filter.Status)
	}
	if filter.From != nil || filter.To != nil {
		t.Fatalf("expected no date bounds, got %+v", filter)
	}
}

func TestParseScheduledTaskLogListFilter_ValidStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/scheduled-tasks/abc/logs?status=success", nil)

	filter, err := parseScheduledTaskLogListFilter(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filter.Status != "success" {
		t.Fatalf("unexpected status: %q", filter.Status)
	}
}

func TestParseScheduledTaskLogListFilter_InvalidStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/scheduled-tasks/abc/logs?status=foobar", nil)

	_, err := parseScheduledTaskLogListFilter(c)
	if err == nil {
		t.Fatal("expected error for invalid status")
	}
}

func TestParseScheduledTaskLogListFilter_ValidDates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/scheduled-tasks/abc/logs?from=2024-01-01T00:00:00Z&to=2024-12-31T23:59:59Z", nil)

	filter, err := parseScheduledTaskLogListFilter(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filter.From == nil || filter.To == nil {
		t.Fatal("expected date bounds")
	}
}

func TestParseScheduledTaskLogListFilter_InvalidFrom(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/scheduled-tasks/abc/logs?from=not-a-date", nil)

	_, err := parseScheduledTaskLogListFilter(c)
	if err == nil {
		t.Fatal("expected error for invalid from")
	}
}

func TestParseScheduledTaskLogListFilter_FromAfterTo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/scheduled-tasks/abc/logs?from=2024-12-31T23:59:59Z&to=2024-01-01T00:00:00Z", nil)

	_, err := parseScheduledTaskLogListFilter(c)
	if err == nil {
		t.Fatal("expected error when from is after to")
	}
}

func TestParseScheduledTaskLogListFilter_DateBoundaryEqual(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ts := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/scheduled-tasks/abc/logs?from="+ts+"&to="+ts, nil)

	_, err := parseScheduledTaskLogListFilter(c)
	if err != nil {
		t.Fatalf("equal from/to should be valid: %v", err)
	}
}
