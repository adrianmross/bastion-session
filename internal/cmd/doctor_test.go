package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
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

func TestDoctorCachedSkipsLiveOCIProbe(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	currentPath := filepath.Join(dir, "current.json")
	includePath := filepath.Join(dir, "ssh", "config.d", "bastion-session")
	if err := os.MkdirAll(filepath.Dir(includePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(includePath, []byte("Host test-bastion\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := app.SaveSession(statePath, app.BastionSession{
		ID:             "ocid1.bastionsession.oc1..s1",
		LifecycleState: "ACTIVE",
		TimeExpires:    time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.SaveCurrent(currentPath, app.CurrentBastion{
		ID:         "ocid1.bastion.oc1..b1",
		Name:       "b1",
		Profile:    "DEFAULT",
		Region:     "us-chicago-1",
		Source:     "test",
		SelectedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"--no-context-scope",
		"--state-path", statePath,
		"--current-path", currentPath,
		"--ssh-include", includePath,
		"doctor", "--cached", "-o", "json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("doctor: %v\n%s", err, out.String())
	}

	var report doctorReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("json output: %v\n%s", err, out.String())
	}
	if report.Session.Live != nil || report.Session.LiveError != "" {
		t.Fatalf("expected cached mode to skip live probe: %#v", report.Session)
	}
}

func TestDoctorReportsExpiredSessionIssue(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	includePath := filepath.Join(dir, "ssh", "config.d", "bastion-session")
	if err := os.MkdirAll(filepath.Dir(includePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(includePath, []byte("Host test-bastion\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := app.SaveSession(statePath, app.BastionSession{
		ID:             "ocid1.bastionsession.oc1..expired",
		LifecycleState: "ACTIVE",
		TimeExpires:    time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"--no-context-scope",
		"--state-path", statePath,
		"--ssh-include", includePath,
		"doctor", "--cached", "-o", "json",
	})
	err := root.Execute()
	if err == nil {
		t.Fatalf("expected doctor error for expired session")
	}
	var exitErr doctorExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 3 {
		t.Fatalf("expected session exit error, got %T %v", err, err)
	}
	var report doctorReport
	if jsonErr := json.Unmarshal(out.Bytes(), &report); jsonErr != nil {
		t.Fatalf("json output: %v\n%s", jsonErr, out.String())
	}
	found := false
	for _, issue := range report.Issues {
		if issue.Code == "cached_session_expired" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected cached_session_expired issue: %#v", report.Issues)
	}
}

func TestDoctorDoesNotRequireTrackedTargetWhenHostIsUsable(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	currentPath := filepath.Join(dir, "current.json")
	includePath := filepath.Join(dir, "ssh", "config.d", "bastion-session")
	trackedTargetsPath := filepath.Join(dir, "tracked-targets.json")
	if err := os.MkdirAll(filepath.Dir(includePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(includePath, []byte("Host test-bastion\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := app.SaveSession(statePath, app.BastionSession{
		ID:               "ocid1.bastionsession.oc1..s1",
		BastionID:        "ocid1.bastion.oc1..b1",
		TargetResourceID: "ocid1.instance.oc1..i1",
		TargetPrivateIP:  "10.42.1.217",
		LifecycleState:   "ACTIVE",
		TimeExpires:      time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.SaveCurrent(currentPath, app.CurrentBastion{
		ID:         "ocid1.bastion.oc1..b1",
		Name:       "b1",
		Profile:    "DEFAULT",
		Region:     "us-chicago-1",
		Source:     "test",
		SelectedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFakeSSH(t, binDir, `#!/bin/sh
if [ "$1" = "-G" ] && [ "$2" = "vmordws02" ]; then
  printf '%s\n' 'hostname 10.42.1.217' 'user opc' 'proxyjump test-bastion'
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
		"--ssh-include", includePath,
		"--tracked-targets-path", trackedTargetsPath,
		"doctor", "vmordws02", "--cached", "-o", "json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("doctor: %v\n%s", err, out.String())
	}

	var report doctorReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("json output: %v\n%s", err, out.String())
	}
	if report.Target != nil {
		t.Fatalf("expected target to remain untracked in report: %#v", report.Target)
	}
	for _, issue := range report.Issues {
		if issue.Code == "tracked_target_missing" {
			t.Fatalf("did not expect tracked_target_missing when host is usable: %#v", report.Issues)
		}
	}
}

func TestDoctorRequiresTrackedTargetWhenHostIsNotUsable(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	currentPath := filepath.Join(dir, "current.json")
	includePath := filepath.Join(dir, "ssh", "config.d", "bastion-session")
	trackedTargetsPath := filepath.Join(dir, "tracked-targets.json")
	if err := os.MkdirAll(filepath.Dir(includePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(includePath, []byte("Host test-bastion\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := app.SaveSession(statePath, app.BastionSession{
		ID:              "ocid1.bastionsession.oc1..s1",
		TargetPrivateIP: "10.42.1.217",
		LifecycleState:  "ACTIVE",
		TimeExpires:     time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := app.SaveCurrent(currentPath, app.CurrentBastion{
		ID:         "ocid1.bastion.oc1..b1",
		Name:       "b1",
		Profile:    "DEFAULT",
		Region:     "us-chicago-1",
		Source:     "test",
		SelectedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFakeSSH(t, binDir, `#!/bin/sh
if [ "$1" = "-G" ] && [ "$2" = "vmordws02" ]; then
  printf '%s\n' 'hostname 10.42.1.99' 'user opc' 'proxyjump test-bastion'
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
		"--ssh-include", includePath,
		"--tracked-targets-path", trackedTargetsPath,
		"doctor", "vmordws02", "--cached", "-o", "json",
	})
	err := root.Execute()
	if err == nil {
		t.Fatalf("expected doctor error for untracked unusable host")
	}
	var report doctorReport
	if jsonErr := json.Unmarshal(out.Bytes(), &report); jsonErr != nil {
		t.Fatalf("json output: %v\n%s", jsonErr, out.String())
	}
	found := false
	for _, issue := range report.Issues {
		if issue.Code == "tracked_target_missing" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected tracked_target_missing issue: %#v", report.Issues)
	}
}

func TestDoctorFixCreatesMissingSSHInclude(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	currentPath := filepath.Join(dir, "current.json")
	includePath := filepath.Join(dir, "ssh", "config.d", "bastion-session")
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"--no-context-scope",
		"--state-path", statePath,
		"--current-path", currentPath,
		"--ssh-include", includePath,
		"doctor", "--fix", "-o", "json",
	})
	if err := root.Execute(); err == nil {
		t.Fatalf("expected remaining current/session issues")
	}
	if _, err := os.Stat(includePath); err != nil {
		t.Fatalf("expected fix to create include file: %v", err)
	}
	var report doctorReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("json output: %v\n%s", err, out.String())
	}
	if len(report.Fixes) == 0 || report.Fixes[0].Code != "ssh_include_ensured" {
		t.Fatalf("expected include fix in report: %#v", report.Fixes)
	}
}
