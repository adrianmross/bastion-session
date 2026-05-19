package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestPathsJSON(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	trackedTargetsPath := filepath.Join(dir, "targets.json")

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"--no-context-scope",
		"--state-path", statePath,
		"--tracked-targets-path", trackedTargetsPath,
		"paths", "-o", "json",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("paths: %v\n%s", err, out.String())
	}
	var result pathsResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("json output: %v\n%s", err, out.String())
	}
	if result.SessionStatePath != statePath || result.TrackedTargetsPath != trackedTargetsPath {
		t.Fatalf("unexpected paths: %#v", result)
	}
}

func TestVersionJSONCommandAndFlag(t *testing.T) {
	for _, args := range [][]string{
		{"version", "-o", "json"},
		{"--version", "--json"},
	} {
		root := newRootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out.String())
		}
		var result cliVersion
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatalf("%v json output: %v\n%s", args, err, out.String())
		}
		if result.Version == "" || result.Commit == "" || result.Date == "" {
			t.Fatalf("%v unexpected version result: %#v", args, result)
		}
	}
}

func TestDoctorWarningOnlyDoesNotFail(t *testing.T) {
	issues := []doctorIssue{
		{Code: "cached_session_near_expiry", Severity: "warning", Message: "session expires soon"},
	}
	if got := doctorErrorIssues(issues); len(got) != 0 {
		t.Fatalf("expected warning-only issues not to be fatal: %#v", got)
	}
}
