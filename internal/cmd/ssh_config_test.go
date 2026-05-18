package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSSHConfigShowJSONUsesSSHEffectiveConfig(t *testing.T) {
	dir := t.TempDir()
	writeFakeSSH(t, dir, `#!/bin/sh
if [ "$1" = "-G" ] && [ "$2" = "vmordws02" ]; then
  cat <<'OUT'
hostname 10.42.1.217
user opc
proxyjump oabcs1-terraform-bastion
identityfile ~/.ssh/vm.key
OUT
  exit 0
fi
echo "unexpected args: $@" >&2
exit 2
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--no-context-scope", "ssh-config", "show", "vmordws02", "-o", "json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("ssh-config show: %v\n%s", err, out.String())
	}

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json output: %v\n%s", err, out.String())
	}
	if got["hostname"] != "10.42.1.217" || got["user"] != "opc" || got["proxyjump"] != "oabcs1-terraform-bastion" || got["identity_file"] != "~/.ssh/vm.key" {
		t.Fatalf("unexpected ssh config: %#v", got)
	}
}

func writeFakeSSH(t *testing.T, dir string, script string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "ssh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}
