package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adrianmross/bastion-session/internal/app"
)

func TestLoadWatchTargetsTrackedUsesTrackedIdentityAndLimit(t *testing.T) {
	dir := t.TempDir()
	trackedPath := filepath.Join(dir, "tracked.json")
	now := time.Now().UTC()
	if err := app.SaveTrackedTargets(trackedPath, []app.TrackedTarget{
		{
			Name:         "first-host",
			BastionID:    "ocid1.bastion.oc1..first",
			InstanceID:   "ocid1.instance.oc1..first",
			PrivateIP:    "10.0.0.10",
			User:         "cloud-user",
			IdentityFile: "/tmp/first",
			Profile:      "FIRST",
			Region:       "us-phoenix-1",
			AuthMethod:   "security_token",
			SSHPublicKey: "/tmp/first.pub",
			LastSeenAt:   now,
		},
		{
			Name:       "second",
			BastionID:  "ocid1.bastion.oc1..second",
			InstanceID: "ocid1.instance.oc1..second",
			PrivateIP:  "10.0.0.11",
			Profile:    "SECOND",
			Region:     "us-ashburn-1",
			LastSeenAt: now.Add(-time.Minute),
		},
	}); err != nil {
		t.Fatalf("SaveTrackedTargets: %v", err)
	}

	base := app.Config{
		Profile:             "BASE",
		Region:              "us-chicago-1",
		AuthMethod:          "api_key",
		TrackedTargetsPath:  trackedPath,
		ScopedContext:       &app.ContextRef{Name: "scoped"},
		ContextScopeEnabled: true,
	}

	targets, err := loadWatchTargets(base, "tracked", 1)
	if err != nil {
		t.Fatalf("loadWatchTargets: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected one bounded target, got %d", len(targets))
	}
	got := targets[0]
	if got.BastionID != "ocid1.bastion.oc1..first" {
		t.Fatalf("unexpected target id: %s", got.BastionID)
	}
	if got.InstanceID != "ocid1.instance.oc1..first" || got.PrivateIP != "10.0.0.10" {
		t.Fatalf("unexpected target details: %#v", got)
	}
	if got.Config.Profile != "FIRST" || got.Config.Region != "us-phoenix-1" || got.Config.AuthMethod != "security_token" {
		t.Fatalf("tracked identity not applied: %#v", got.Config)
	}
	if got.Config.TargetUser != "cloud-user" || got.Config.SSHPrivateKey != "/tmp/first" {
		t.Fatalf("tracked target SSH details not applied: %#v", got.Config)
	}
	if got.Config.ScopedContext != nil || got.Config.ContextScopeEnabled {
		t.Fatalf("tracked target should not inherit scoped context: %#v", got.Config.ScopedContext)
	}
	if got.HostAlias != "first-host-bastion" {
		t.Fatalf("unexpected host alias: %s", got.HostAlias)
	}
}

func TestRunWatchIterationTrackedLogsFailuresAndWritesSuccessfulHosts(t *testing.T) {
	dir := t.TempDir()
	trackedPath := filepath.Join(dir, "tracked.json")
	includePath := filepath.Join(dir, "ssh", "config.d", "bastion-session")
	now := time.Now().UTC()
	if err := app.SaveTrackedTargets(trackedPath, []app.TrackedTarget{
		{Name: "ok", BastionID: "ocid1.bastion.oc1..ok", InstanceID: "ocid1.instance.oc1..ok", PrivateIP: "10.0.0.20", Profile: "OK", Region: "us-phoenix-1", AuthMethod: "security_token", LastSeenAt: now},
		{Name: "bad", BastionID: "ocid1.bastion.oc1..bad", InstanceID: "ocid1.instance.oc1..bad", PrivateIP: "10.0.0.21", Profile: "BAD", Region: "us-ashburn-1", AuthMethod: "api_key", LastSeenAt: now.Add(-time.Minute)},
	}); err != nil {
		t.Fatalf("SaveTrackedTargets: %v", err)
	}
	base := app.Config{
		TrackedTargetsPath: trackedPath,
		SSHIncludePath:     includePath,
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	results, err := runWatchIteration(base, "tracked", 10, func(_ app.Config, opts app.RefreshOptions) (app.BastionSession, error) {
		if opts.BastionID == "ocid1.bastion.oc1..ok" && (opts.InstanceID != "ocid1.instance.oc1..ok" || opts.PrivateIP != "10.0.0.20") {
			t.Fatalf("refresh missing target details: %#v", opts)
		}
		if opts.BastionID == "ocid1.bastion.oc1..bad" {
			return app.BastionSession{}, errors.New("OCI CLI reported a security token authentication failure")
		}
		return app.BastionSession{
			ID:             "ocid1.session.oc1..ok",
			LifecycleState: "ACTIVE",
			TimeExpires:    time.Date(2026, 3, 19, 13, 0, 0, 0, time.UTC),
		}, nil
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("runWatchIteration: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one successful refresh, got %d", len(results))
	}
	errLog := stderr.String()
	if !strings.Contains(errLog, "profile=BAD") || !strings.Contains(errLog, "region=us-ashburn-1") || !strings.Contains(errLog, "auth=api_key") {
		t.Fatalf("failure log missing identity context: %s", errLog)
	}
	data, err := os.ReadFile(includePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "Host ok-bastion") || !strings.Contains(content, "User ocid1.session.oc1..ok") {
		t.Fatalf("successful host not written: %s", content)
	}
	if strings.Contains(content, "bad-bastion") {
		t.Fatalf("failed host should not be written: %s", content)
	}
}

func TestLoadWatchTargetsCurrentWorksWithoutSelection(t *testing.T) {
	dir := t.TempDir()
	base := app.Config{
		Profile:          "BASE",
		Region:           "us-chicago-1",
		CurrentStatePath: filepath.Join(dir, "missing-current.json"),
	}

	targets, err := loadWatchTargets(base, "current", 10)
	if err != nil {
		t.Fatalf("loadWatchTargets: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected current fallback target, got %d", len(targets))
	}
	if targets[0].BastionID != "" || targets[0].Config.Profile != "BASE" || targets[0].HostAlias != "BASE-bastion" {
		t.Fatalf("unexpected fallback target: %#v", targets[0])
	}
}
