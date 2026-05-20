package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateSessionPassesSessionTTL(t *testing.T) {
	tmp := t.TempDir()
	argsPath := filepath.Join(tmp, "args")
	ociPath := filepath.Join(tmp, "oci")
	script := `#!/bin/sh
printf '%s\n' "$@" > "` + argsPath + `"
cat <<'JSON'
{"id":"ocid1.session.oc1..s1","bastionId":"ocid1.bastion.oc1..b1","targetResourceId":"ocid1.instance.oc1..i1","targetResourceDetails":{"privateIpAddress":"10.0.0.44"},"lifecycleState":"CREATING","timeCreated":"2026-03-19T12:00:00Z","timeExpires":"2026-03-19T15:00:00Z"}
JSON
`
	if err := os.WriteFile(ociPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tmp+":"+os.Getenv("PATH"))

	_, err := (OCIClient{Profile: "DEFAULT"}).CreateSession(TargetDetails{
		BastionID:     "ocid1.bastion.oc1..b1",
		InstanceID:    "ocid1.instance.oc1..i1",
		PrivateIP:     "10.0.0.44",
		TargetUser:    "opc",
		PublicKeyPath: "/tmp/key.pub",
		SessionTTL:    3 * time.Hour,
	})
	if err != nil {
		t.Fatal(err)
	}
	argsBytes, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSpace(string(argsBytes)), "\n")
	for i, arg := range args {
		if arg == "--session-ttl-in-seconds" {
			if i+1 >= len(args) || args[i+1] != "10800" {
				t.Fatalf("unexpected ttl args: %v", args)
			}
			return
		}
	}
	t.Fatalf("missing --session-ttl-in-seconds in args: %v", args)
}
