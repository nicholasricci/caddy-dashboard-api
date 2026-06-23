package models

import "testing"

func TestIsValidAuditAction(t *testing.T) {
	if !IsValidAuditAction("sync") {
		t.Fatal("expected sync to be valid")
	}
	if IsValidAuditAction("foobar") {
		t.Fatal("expected foobar to be invalid")
	}
}

func TestIsValidAuditResource(t *testing.T) {
	if !IsValidAuditResource("node") {
		t.Fatal("expected node to be valid")
	}
	if IsValidAuditResource("unknown") {
		t.Fatal("expected unknown to be invalid")
	}
}

func TestAuditActionsReturnsCopy(t *testing.T) {
	a := AuditActions()
	a[0] = "mutated"
	if AuditActions()[0] == "mutated" {
		t.Fatal("AuditActions should return a copy")
	}
}

func TestAuditResourcesReturnsCopy(t *testing.T) {
	r := AuditResources()
	r[0] = "mutated"
	if AuditResources()[0] == "mutated" {
		t.Fatal("AuditResources should return a copy")
	}
}
