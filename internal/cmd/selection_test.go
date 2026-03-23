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
