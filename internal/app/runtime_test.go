package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAutoRefreshIntervalClamp(t *testing.T) {
	now := time.Now().UTC()
	s := BastionSession{TimeExpires: now.Add(10 * time.Second)}
	got := AutoRefreshInterval(s)
	if got != MinAutoRefresh {
		t.Fatalf("expected %v, got %v", MinAutoRefresh, got)
	}
}

func TestExtractPathsFromTFVars(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "env.tfvars")
	content := "bastion_ssh_public_key_path = \"ssh/bastion.pub\"\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	vals := extractPathsFromTFVars(p)
	if len(vals) != 1 || vals[0] != "ssh/bastion.pub" {
		t.Fatalf("unexpected values: %#v", vals)
	}
}

func TestResolvePublicKeyFromOutputsTFVars(t *testing.T) {
	dir := t.TempDir()
	keyDir := filepath.Join(dir, "ssh")
	if err := os.MkdirAll(keyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pub := filepath.Join(keyDir, "bastion.pub")
	if err := os.WriteFile(pub, []byte("ssh-rsa AAA"), 0o600); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(dir, "terraform.tfstate")
	stateJSON := `{"outputs":{"bastion_id":{"value":"ocid1.bastion"},"instance_id":{"value":"ocid1.instance"},"private_ip":{"value":"10.0.0.5"}}}`
	if err := os.WriteFile(statePath, []byte(stateJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "env.tfvars"), []byte("bastion_ssh_public_key_path = \"ssh/bastion.pub\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Config{TerraformOutputsPath: statePath}
	if got := ResolvePublicKey(cfg); got != pub {
		t.Fatalf("expected %s, got %s", pub, got)
	}
}
