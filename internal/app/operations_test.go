package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTargetMetadataFromCachedSession(t *testing.T) {
	sessionPath := filepath.Join(t.TempDir(), "session.json")
	if err := SaveSession(sessionPath, BastionSession{
		ID:               "ocid1.session.oc1..cached",
		BastionID:        "ocid1.bastion.oc1..b1",
		TargetResourceID: "ocid1.instance.oc1..i1",
		TargetPrivateIP:  "10.0.0.11",
	}); err != nil {
		t.Fatal(err)
	}
	cfg := Config{SessionStatePath: sessionPath}
	md, err := resolveTargetMetadata(cfg, OCIClient{}, RefreshOptions{
		BastionID: "ocid1.bastion.oc1..b1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if md.BastionID != "ocid1.bastion.oc1..b1" || md.InstanceID != "ocid1.instance.oc1..i1" || md.PrivateIP != "10.0.0.11" {
		t.Fatalf("unexpected metadata: %#v", md)
	}
}

func TestResolveTargetMetadataFromLiveSession(t *testing.T) {
	tmp := t.TempDir()
	sessionPath := filepath.Join(tmp, "session.json")
	if err := SaveSession(sessionPath, BastionSession{
		ID:        "ocid1.session.oc1..live",
		BastionID: "ocid1.bastion.oc1..b1",
	}); err != nil {
		t.Fatal(err)
	}

	ociPath := filepath.Join(tmp, "oci")
	script := `#!/bin/sh
if [ "$1" = "--profile" ]; then shift 2; fi
if [ "$1" = "--region" ]; then shift 2; fi
if [ "$1" = "--auth" ]; then shift 2; fi
if [ "$1" = "bastion" ] && [ "$2" = "session" ] && [ "$3" = "get" ]; then
  cat <<'JSON'
{"id":"ocid1.session.oc1..live","bastionId":"ocid1.bastion.oc1..b1","targetResourceId":"ocid1.instance.oc1..i1","targetResourceDetails":{"privateIpAddress":"10.0.0.12"},"lifecycleState":"ACTIVE","timeCreated":"2026-03-19T12:00:00Z","timeExpires":"2026-03-19T13:00:00Z"}
JSON
  exit 0
fi
echo "unexpected args: $@" >&2
exit 1
`
	if err := os.WriteFile(ociPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tmp+":"+os.Getenv("PATH"))

	cfg := Config{
		SessionStatePath: sessionPath,
		Profile:          "DEFAULT",
	}
	md, err := resolveTargetMetadata(cfg, OCIClient{Profile: "DEFAULT"}, RefreshOptions{
		BastionID: "ocid1.bastion.oc1..b1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if md.InstanceID != "ocid1.instance.oc1..i1" || md.PrivateIP != "10.0.0.12" {
		t.Fatalf("unexpected metadata from live session: %#v", md)
	}
}
