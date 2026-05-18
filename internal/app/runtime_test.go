package app

import (
	"os"
	"path/filepath"
	"strings"
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

func TestResolvePublicKeyFallsBackToDefaultSSHKey(t *testing.T) {
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pub := filepath.Join(sshDir, "id_ed25519.pub")
	if err := os.WriteFile(pub, []byte("ssh-ed25519 AAA"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("SSH_PUBLIC_KEY", "")
	t.Setenv("TF_VAR_bastion_ssh_public_key_path", "")
	t.Setenv("TF_VAR_ssh_public_key_path", "")
	t.Setenv("BASTION_SSH_PUBLIC_KEY_PATH", "")
	t.Setenv("SSH_PUBLIC_KEY_PATH", "")

	if got := ResolvePublicKey(Config{}); got != pub {
		t.Fatalf("expected %s, got %s", pub, got)
	}
}

func TestUpdateSSHFragmentWithTargetWritesVMHostAlias(t *testing.T) {
	dir := t.TempDir()
	include := filepath.Join(dir, "config.d", "bastion-session")
	cfg := Config{
		Profile:        "dev",
		Region:         "us-chicago-1",
		TargetUser:     "opc",
		SSHPublicKey:   filepath.Join(dir, "bastion.key.pub"),
		SSHIncludePath: include,
	}
	if err := os.WriteFile(filepath.Join(dir, "bastion.key"), []byte("PRIVATE"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.SSHPublicKey, []byte("PUBLIC"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := UpdateSSHFragmentWithTarget(cfg, "ocid1.bastionsession.oc1..abc", TargetSSHHost{
		Alias:        "vmordws02",
		HostName:     "10.42.1.217",
		IdentityFile: "~/.ssh/vm.key",
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(include)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"Host dev-bastion",
		"  User ocid1.bastionsession.oc1..abc",
		"  IdentityFile " + filepath.Join(dir, "bastion.key"),
		"Host vmordws02",
		"  HostName 10.42.1.217",
		"  User opc",
		"  IdentityFile ~/.ssh/vm.key",
		"  ProxyJump dev-bastion",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected fragment to contain %q, got:\n%s", want, got)
		}
	}
}

func TestUpdateSSHFragmentPreservesTargetAliasesAcrossSessionRefresh(t *testing.T) {
	dir := t.TempDir()
	include := filepath.Join(dir, "config.d", "bastion-session")
	cfg := Config{
		Profile:        "dev",
		Region:         "us-chicago-1",
		TargetUser:     "opc",
		SSHIncludePath: include,
	}
	if err := UpdateSSHFragmentWithTarget(cfg, "ocid1.session.old", TargetSSHHost{
		Alias:    "vmordws02",
		HostName: "10.42.1.217",
	}); err != nil {
		t.Fatal(err)
	}
	if err := UpdateSSHFragment(cfg, "ocid1.session.new"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(include)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		"  User ocid1.session.new",
		"Host vmordws02",
		"  HostName 10.42.1.217",
		"  ProxyJump dev-bastion",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected refreshed fragment to contain %q, got:\n%s", want, got)
		}
	}
	if strings.Contains(got, "ocid1.session.old") {
		t.Fatalf("old session ID should not remain in bastion block:\n%s", got)
	}
}
