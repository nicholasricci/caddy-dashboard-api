package services

import (
	"testing"
)

func TestBuildRegisterDial(t *testing.T) {
	dial, err := buildRegisterDial("10.0.0.5", 8080, "")
	if err != nil {
		t.Fatalf("buildRegisterDial: %v", err)
	}
	if dial != "10.0.0.5:8080" {
		t.Fatalf("dial=%q", dial)
	}
	dial, err = buildRegisterDial("", 0, "10.0.0.6:9090")
	if err != nil || dial != "10.0.0.6:9090" {
		t.Fatalf("override dial: %v %q", err, dial)
	}
	if _, err := buildRegisterDial("", 0, ""); err == nil {
		t.Fatal("expected invalid dial error")
	}
}
