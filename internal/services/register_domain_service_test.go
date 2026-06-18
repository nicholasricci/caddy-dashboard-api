package services

import (
	"testing"
)

func TestNormalizeRegisterDomains(t *testing.T) {
	domains, err := normalizeRegisterDomains([]string{" b.example.com ", "a.example.com", "b.example.com"})
	if err != nil {
		t.Fatalf("normalizeRegisterDomains: %v", err)
	}
	if len(domains) != 2 || domains[0] != "a.example.com" || domains[1] != "b.example.com" {
		t.Fatalf("domains=%v", domains)
	}
	if _, err := normalizeRegisterDomains(nil); err == nil {
		t.Fatal("expected empty domains error")
	}
	if _, err := normalizeRegisterDomains([]string{"", "  "}); err == nil {
		t.Fatal("expected invalid domains error")
	}
}
