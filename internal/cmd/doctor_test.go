package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adrianmross/bastion-session/internal/app"
)

func TestDoctorJSONDoesNotRequireLiveOCI(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	currentPath := filepath.Join(dir, "current.json")
	trackedPath := filepath.Join(dir, "tracked.json")
	trackedTargetsPath := filepath.Join(dir, "tracked-targets.json")
	includePath := filepath.Join(dir, "ssh", "config.d", "bastion-session")
	if err := os.MkdirAll(filepath.Dir(includePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(includePath, []byte("Host oabcs1-terraform-bastion\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour).UTC()
	if err := app.SaveSession(statePath, app.BastionSession{
		ID:               "ocid1.bastionsession.oc1..s1",
		BastionID:        "ocid1.bastion.oc1..b1",
		TargetResourceID: "ocid1.instance.oc1..i1",
		TargetPrivateIP:  "10.42.1.217",
		LifecycleState:   "ACTIVE",
		TimeExpires:      expires,
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.SaveCurrent(currentPath, app.CurrentBastion{
		ID:         "ocid1.bastion.oc1..b1",
		Name:       "b1",
		Profile:    "DEFAULT",
		Region:     "us-chicago-1",
		Source:     "test",
		SelectedAt: time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.SaveTracked(trackedPath, []app.TrackedBastion{{ID: "ocid1.bastion.oc1..b1"}}); err != nil {
		t.Fatal(err)
	}
	if err := app.SaveTrackedTargets(trackedTargetsPath, []app.TrackedTarget{{
		Name:       "vmordws02",
		InstanceID: "ocid1.instance.oc1..i1",
		PrivateIP:  "10.42.1.217",
		BastionID:  "ocid1.bastion.oc1..b1",
		User:       "opc",
	}}); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFakeSSH(t, binDir, `#!/bin/sh
if [ "$1" = "-G" ] && [ "$2" = "vmordws02" ]; then
  printf '%s\n' 'hostname 10.42.1.217' 'user opc' 'proxyjump oabcs1-terraform-bastion' 'identityfile ~/.ssh/vm.key'
  exit 0
fi
exit 2
`)
	t.Setenv("PATH", binDir)

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"--no-context-scope",
		"--state-path", statePath,
		"--current-path", currentPath,
		"--tracked-path", trackedPath,
		"--tracked-targets-path", trackedTargetsPath,
		"--ssh-include", includePath,
		"doctor", "vmordws02", "-o", "json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("doctor: %v\n%s", err, out.String())
	}

	var report doctorReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("json output: %v\n%s", err, out.String())
	}
	if !report.Current.Available {
		t.Fatalf("expected current bastion to be available: %#v", report.Current)
	}
	if report.Target == nil || report.Target.PrivateIP != "10.42.1.217" {
		t.Fatalf("expected tracked target in report: %#v", report.Target)
	}
	if report.Session.Cached == nil || report.Session.Cached.ID != "ocid1.bastionsession.oc1..s1" {
		t.Fatalf("expected cached session: %#v", report.Session)
	}
	if report.Session.LiveError == "" {
		t.Fatalf("expected live OCI error to be captured")
	}
	if !report.SSHInclude.Exists {
		t.Fatalf("expected ssh include to exist")
	}
	if report.SSHConfig == nil || report.SSHConfig.ProxyJump != "oabcs1-terraform-bastion" {
		t.Fatalf("expected parsed ssh config: %#v", report.SSHConfig)
	}
}
