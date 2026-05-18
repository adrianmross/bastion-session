package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adrianmross/bastion-session/internal/app"
)

func TestTargetTrackFromTerraformOutputs(t *testing.T) {
	dir := t.TempDir()
	trackedPath := filepath.Join(dir, "tracked-targets.json")
	outputsPath := filepath.Join(dir, "outputs.json")
	outputs := `{"bastion_id":{"value":"ocid1.bastion.oc1..b1"},"instance_id":{"value":"ocid1.instance.oc1..i1"},"private_ip":{"value":"10.42.1.217"}}`
	if err := os.WriteFile(outputsPath, []byte(outputs), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"--no-context-scope",
		"--tracked-targets-path", trackedPath,
		"target", "track", "vmordws02",
		"--terraform-outputs", outputsPath,
		"--user", "opc",
		"--identity-file", "~/.ssh/vm.key",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("target track: %v\n%s", err, out.String())
	}

	target, err := app.FindTrackedTarget(trackedPath, "vmordws02")
	if err != nil {
		t.Fatal(err)
	}
	if target == nil {
		t.Fatalf("expected tracked target")
	}
	if target.InstanceID != "ocid1.instance.oc1..i1" {
		t.Fatalf("unexpected instance ID: %s", target.InstanceID)
	}
	if target.PrivateIP != "10.42.1.217" {
		t.Fatalf("unexpected private IP: %s", target.PrivateIP)
	}
	if target.BastionID != "ocid1.bastion.oc1..b1" {
		t.Fatalf("unexpected bastion ID: %s", target.BastionID)
	}
	if target.User != "opc" {
		t.Fatalf("unexpected user: %s", target.User)
	}
	if target.IdentityFile != "~/.ssh/vm.key" {
		t.Fatalf("unexpected identity file: %s", target.IdentityFile)
	}
}

func TestTargetImportFromTerraformDirectoryWithOverrides(t *testing.T) {
	dir := t.TempDir()
	trackedPath := filepath.Join(dir, "tracked-targets.json")
	tfDir := filepath.Join(dir, "tf")
	if err := os.MkdirAll(tfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(tfDir, "terraform.tfstate")
	state := `{"outputs":{"bastion_id":{"value":"ocid1.bastion.oc1..fromtf"},"instance_id":{"value":"ocid1.instance.oc1..i2"},"private_ip":{"value":"10.42.1.218"}}}`
	if err := os.WriteFile(statePath, []byte(state), 0o600); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"--no-context-scope",
		"--tracked-targets-path", trackedPath,
		"target", "import", "vmordws03",
		"--terraform-outputs", tfDir,
		"--user", "ubuntu",
		"--identity-file", "~/.ssh/imported.key",
		"--bastion-id", "ocid1.bastion.oc1..override",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("target import: %v\n%s", err, out.String())
	}

	target, err := app.FindTrackedTarget(trackedPath, "vmordws03")
	if err != nil {
		t.Fatal(err)
	}
	if target == nil {
		t.Fatalf("expected tracked target")
	}
	for got, want := range map[string]string{
		target.InstanceID:           "ocid1.instance.oc1..i2",
		target.PrivateIP:            "10.42.1.218",
		target.BastionID:            "ocid1.bastion.oc1..override",
		target.User:                 "ubuntu",
		target.IdentityFile:         "~/.ssh/imported.key",
		target.TerraformOutputsPath: statePath,
	} {
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	}
}

func TestApplyTrackedTargetForEnsureFillsMissingValues(t *testing.T) {
	cfg := app.Config{TargetUser: app.DefaultTargetUser}
	target := &app.TrackedTarget{
		Name:                 "vmordws02",
		InstanceID:           "ocid1.instance.oc1..i1",
		PrivateIP:            "10.42.1.217",
		User:                 "ubuntu",
		IdentityFile:         "~/.ssh/vm.key",
		BastionID:            "ocid1.bastion.oc1..b1",
		TerraformOutputsPath: "/tmp/outputs.json",
	}
	bastionID := ""
	instanceID := ""
	privateIP := ""
	identityFile := ""

	applyTrackedTargetForEnsure(&cfg, target, &bastionID, &instanceID, &privateIP, &identityFile, false)

	if bastionID != target.BastionID {
		t.Fatalf("expected bastion ID from target, got %s", bastionID)
	}
	if instanceID != target.InstanceID {
		t.Fatalf("expected instance ID from target, got %s", instanceID)
	}
	if privateIP != target.PrivateIP {
		t.Fatalf("expected private IP from target, got %s", privateIP)
	}
	if identityFile != target.IdentityFile {
		t.Fatalf("expected identity file from target, got %s", identityFile)
	}
	if cfg.TargetUser != "ubuntu" {
		t.Fatalf("expected target user from target, got %s", cfg.TargetUser)
	}
	if cfg.TerraformOutputsPath != "/tmp/outputs.json" {
		t.Fatalf("expected terraform outputs path from target, got %s", cfg.TerraformOutputsPath)
	}
}

func TestApplyTrackedTargetForEnsurePreservesExplicitValues(t *testing.T) {
	cfg := app.Config{TargetUser: "opc"}
	target := &app.TrackedTarget{
		InstanceID:   "tracked-instance",
		PrivateIP:    "10.0.0.1",
		User:         "ubuntu",
		IdentityFile: "tracked-key",
		BastionID:    "tracked-bastion",
	}
	bastionID := "explicit-bastion"
	instanceID := "explicit-instance"
	privateIP := "10.0.0.99"
	identityFile := "explicit-key"

	applyTrackedTargetForEnsure(&cfg, target, &bastionID, &instanceID, &privateIP, &identityFile, true)

	for got, want := range map[string]string{
		bastionID:      "explicit-bastion",
		instanceID:     "explicit-instance",
		privateIP:      "10.0.0.99",
		identityFile:   "explicit-key",
		cfg.TargetUser: "opc",
	} {
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	}
}

func TestTargetRmFailsWhenNameDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	trackedPath := filepath.Join(dir, "tracked-targets.json")
	if err := app.SaveTrackedTargets(trackedPath, []app.TrackedTarget{{Name: "vmordws02", InstanceID: "i", PrivateIP: "10.0.0.1"}}); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--no-context-scope", "--tracked-targets-path", trackedPath, "target", "rm", "missing"})
	err := root.Execute()
	if err == nil {
		t.Fatalf("expected target rm to fail")
	}
	if !strings.Contains(err.Error(), `tracked target "missing" not found`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
