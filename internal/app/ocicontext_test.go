package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveOCIContextConfigPath(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(dir, ".oci-context.yml")
	if err := os.WriteFile(local, []byte("current_context: \"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := ResolveOCIContextConfigPath("", false)
	if err != nil {
		t.Fatal(err)
	}
	gotAbs, _ := filepath.Abs(p)
	expAbs, _ := filepath.Abs(local)
	gotReal, _ := filepath.EvalSymlinks(gotAbs)
	expReal, _ := filepath.EvalSymlinks(expAbs)
	if gotReal != expReal {
		t.Fatalf("expected %s, got %s", expReal, gotReal)
	}
}

func TestLoadCurrentOCIContext(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yml")
	yaml := `current_context: dev
contexts:
  - name: dev
    profile: DEFAULT
    auth_method: security_token
    tenancy_ocid: ocid1.tenancy
    compartment_ocid: ocid1.compartment
    region: us-phoenix-1
    user: ocid1.user
`
	if err := os.WriteFile(p, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, err := LoadCurrentOCIContext(p)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Name != "dev" || ctx.Profile != "DEFAULT" || ctx.AuthMethod != "security_token" || ctx.CompartmentOCID != "ocid1.compartment" {
		t.Fatalf("unexpected context: %#v", ctx)
	}
}
