package app

import (
	"path/filepath"
	"testing"
	"time"
)

func TestUpsertTrackedTargetPreservesExistingOptionalFields(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tracked-targets.json")
	first := TrackedTarget{
		Name:         "vmordws02",
		InstanceID:   "ocid1.instance.oc1..old",
		PrivateIP:    "10.42.1.217",
		User:         "opc",
		IdentityFile: "~/.ssh/vm.key",
		BastionID:    "ocid1.bastion.oc1..b1",
		LastSeenAt:   time.Now().Add(-time.Hour),
	}
	if err := UpsertTrackedTarget(p, first); err != nil {
		t.Fatal(err)
	}
	if err := UpsertTrackedTarget(p, TrackedTarget{
		Name:       "vmordws02",
		InstanceID: "ocid1.instance.oc1..new",
		PrivateIP:  "10.42.1.218",
		LastSeenAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	got, err := FindTrackedTarget(p, "vmordws02")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatalf("expected tracked target")
	}
	if got.InstanceID != "ocid1.instance.oc1..new" {
		t.Fatalf("expected updated instance ID, got %s", got.InstanceID)
	}
	if got.PrivateIP != "10.42.1.218" {
		t.Fatalf("expected updated private IP, got %s", got.PrivateIP)
	}
	if got.User != "opc" {
		t.Fatalf("expected user to be preserved, got %s", got.User)
	}
	if got.IdentityFile != "~/.ssh/vm.key" {
		t.Fatalf("expected identity file to be preserved, got %s", got.IdentityFile)
	}
	if got.BastionID != "ocid1.bastion.oc1..b1" {
		t.Fatalf("expected bastion ID to be preserved, got %s", got.BastionID)
	}
}

func TestRemoveTrackedTarget(t *testing.T) {
	p := filepath.Join(t.TempDir(), "tracked-targets.json")
	if err := SaveTrackedTargets(p, []TrackedTarget{
		{Name: "one", InstanceID: "i1", PrivateIP: "10.0.0.1"},
		{Name: "two", InstanceID: "i2", PrivateIP: "10.0.0.2"},
	}); err != nil {
		t.Fatal(err)
	}

	removed, err := RemoveTrackedTarget(p, "one")
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}
	if got, err := FindTrackedTarget(p, "one"); err != nil {
		t.Fatal(err)
	} else if got != nil {
		t.Fatalf("expected target to be removed: %#v", got)
	}
}
