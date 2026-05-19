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

func TestSessionPruneRemovesExpiredCachedSession(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	if err := app.SaveSession(statePath, app.BastionSession{
		ID:             "ocid1.bastionsession.oc1..expired",
		LifecycleState: "ACTIVE",
		TimeExpires:    time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--no-context-scope", "--state-path", statePath, "session", "prune", "-o", "json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("session prune: %v\n%s", err, out.String())
	}
	var result app.SessionPruneResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("json output: %v\n%s", err, out.String())
	}
	if !result.Pruned {
		t.Fatalf("expected session pruned: %#v", result)
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("expected state file removed, stat err=%v", err)
	}
}

func TestSessionPruneKeepsUnexpiredCachedSession(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	if err := app.SaveSession(statePath, app.BastionSession{
		ID:             "ocid1.bastionsession.oc1..active",
		LifecycleState: "ACTIVE",
		TimeExpires:    time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--no-context-scope", "--state-path", statePath, "session", "prune", "-o", "json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("session prune: %v\n%s", err, out.String())
	}
	var result app.SessionPruneResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("json output: %v\n%s", err, out.String())
	}
	if result.Pruned {
		t.Fatalf("did not expect active session to be pruned: %#v", result)
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("expected state file to remain: %v", err)
	}
}
