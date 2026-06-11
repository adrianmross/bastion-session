package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEffectiveSessionTTLClampsToBastionMax(t *testing.T) {
	tmp := t.TempDir()
	ociPath := filepath.Join(tmp, "oci")
	script := `#!/bin/sh
if [ "$1" = "--profile" ]; then shift 2; fi
if [ "$1" = "--region" ]; then shift 2; fi
if [ "$1" = "--auth" ]; then shift 2; fi
if [ "$1" = "bastion" ] && [ "$2" = "bastion" ] && [ "$3" = "get" ]; then
  printf '{"id":"ocid1.bastion.oc1..b1","maxSessionTtlInSeconds":10800}\n'
  exit 0
fi
echo "unexpected args: $@" >&2
exit 1
`
	if err := os.WriteFile(ociPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tmp+":"+os.Getenv("PATH"))

	got := effectiveSessionTTL(OCIClient{Profile: "DEFAULT"}, "ocid1.bastion.oc1..b1", 24*time.Hour)
	if got != 3*time.Hour {
		t.Fatalf("effectiveSessionTTL=%s, want 3h", got)
	}
}

func TestEffectiveSessionTTLKeepsRequestedWithinMax(t *testing.T) {
	tmp := t.TempDir()
	ociPath := filepath.Join(tmp, "oci")
	script := `#!/bin/sh
if [ "$1" = "--profile" ]; then shift 2; fi
if [ "$1" = "bastion" ] && [ "$2" = "bastion" ] && [ "$3" = "get" ]; then
  printf '{"id":"ocid1.bastion.oc1..b1","max-session-ttl-in-seconds":86400}\n'
  exit 0
fi
echo "unexpected args: $@" >&2
exit 1
`
	if err := os.WriteFile(ociPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tmp+":"+os.Getenv("PATH"))

	got := effectiveSessionTTL(OCIClient{Profile: "DEFAULT"}, "ocid1.bastion.oc1..b1", 3*time.Hour)
	if got != 3*time.Hour {
		t.Fatalf("effectiveSessionTTL=%s, want 3h", got)
	}
}
