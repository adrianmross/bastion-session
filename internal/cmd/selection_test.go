package cmd

import (
	"path/filepath"
	"testing"

	"github.com/adrianmross/bastion-session/internal/app"
)

func TestLoadCurrentSelectionKeepsScopedConfigAuthoritative(t *testing.T) {
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.json")
	if err := app.SaveCurrent(currentPath, app.CurrentBastion{
		ID:           "ocid1.bastion.oc1..x",
		Profile:      "DEFAULT",
		Region:       "us-sanjose-1",
		AuthMethod:   "api_key",
		SSHPublicKey: "/tmp/id.pub",
		ContextName:  "oabcs1-terraform",
	}); err != nil {
		t.Fatalf("SaveCurrent: %v", err)
	}

	cfg := app.Config{
		CurrentStatePath: currentPath,
		Profile:          "oabcs1-terraform",
		Region:           "us-chicago-1",
		AuthMethod:       "security_token",
		ScopedContext: &app.ContextRef{
			Name:       "oabcs1-terraform",
			Profile:    "oabcs1-terraform",
			Region:     "us-chicago-1",
			AuthMethod: "security_token",
		},
	}

	cur, err := loadCurrentSelection(&cfg)
	if err != nil {
		t.Fatalf("loadCurrentSelection: %v", err)
	}
	if cur == nil {
		t.Fatalf("expected current selection")
	}
	if cfg.Profile != "oabcs1-terraform" {
		t.Fatalf("profile overridden unexpectedly: %q", cfg.Profile)
	}
	if cfg.Region != "us-chicago-1" {
		t.Fatalf("region overridden unexpectedly: %q", cfg.Region)
	}
	if cfg.AuthMethod != "security_token" {
		t.Fatalf("auth_method overridden unexpectedly: %q", cfg.AuthMethod)
	}
	if cfg.SSHPublicKey != "/tmp/id.pub" {
		t.Fatalf("ssh key should still backfill: %q", cfg.SSHPublicKey)
	}
}

func TestResolveBastionIDTokenFromTrackedRef(t *testing.T) {
	dir := t.TempDir()
	trackedPath := filepath.Join(dir, "tracked.json")
	currentPath := filepath.Join(dir, "current.json")
	items := []app.TrackedBastion{
		{ID: "ocid1.bastion.oc1..alpha", Name: "alpha"},
		{ID: "ocid1.bastion.oc1..beta", Name: "beta"},
	}
	if err := app.SaveTracked(trackedPath, items); err != nil {
		t.Fatalf("SaveTracked: %v", err)
	}
	refs := app.BuildShortRefs([]string{items[0].ID, items[1].ID}, 2)
	cfg := app.Config{
		TrackedBastionsPath: trackedPath,
		CurrentStatePath:    currentPath,
		ContextScopeEnabled: false,
	}

	got, err := resolveBastionIDToken(&cfg, refs[items[1].ID])
	if err != nil {
		t.Fatalf("resolveBastionIDToken: %v", err)
	}
	if got != items[1].ID {
		t.Fatalf("expected %s, got %s", items[1].ID, got)
	}
}

func TestResolveBastionIDTokenFromCurrentRef(t *testing.T) {
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.json")
	if err := app.SaveCurrent(currentPath, app.CurrentBastion{
		ID: "ocid1.bastion.oc1..current",
	}); err != nil {
		t.Fatalf("SaveCurrent: %v", err)
	}
	ref := app.BuildShortRefs([]string{"ocid1.bastion.oc1..current"}, 2)["ocid1.bastion.oc1..current"]
	cfg := app.Config{
		TrackedBastionsPath: filepath.Join(dir, "tracked.json"),
		CurrentStatePath:    currentPath,
		ContextScopeEnabled: false,
	}

	got, err := resolveBastionIDToken(&cfg, ref)
	if err != nil {
		t.Fatalf("resolveBastionIDToken: %v", err)
	}
	if got != "ocid1.bastion.oc1..current" {
		t.Fatalf("expected current id, got %s", got)
	}
}
