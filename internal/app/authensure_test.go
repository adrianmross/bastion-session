package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureOCIContextAuthRunsForScopedContext(t *testing.T) {
	tmp := t.TempDir()
	argsPath := filepath.Join(tmp, "args")
	binPath := filepath.Join(tmp, "oci-context")
	script := `#!/bin/sh
printf '%s\n' "$@" > "` + argsPath + `"
printf '{"ok":true,"state":"ready"}\n'
`
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tmp+":"+os.Getenv("PATH"))

	err := EnsureOCIContextAuth(Config{
		ContextScopeEnabled:  true,
		OCIContextConfigPath: "/tmp/oci-context.yml",
		OCIContextName:       "dev",
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := []string{"auth", "--config", "/tmp/oci-context.yml", "--context", "dev", "--no-interactive", "ensure", "--output", "json"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("unexpected oci-context args: %v", got)
	}
}

func TestEnsureOCIContextAuthSkipsWhenUnscoped(t *testing.T) {
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "oci-context")
	script := `#!/bin/sh
exit 99
`
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tmp+":"+os.Getenv("PATH"))

	if err := EnsureOCIContextAuth(Config{}); err != nil {
		t.Fatal(err)
	}
}
