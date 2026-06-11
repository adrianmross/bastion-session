package app

import (
	"os"
	"path/filepath"
	"slices"
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
		if arg == "--session-ttl" {
			if i+1 >= len(args) || args[i+1] != "10800" {
				t.Fatalf("unexpected ttl args: %v", args)
			}
			return
		}
	}
	t.Fatalf("missing --session-ttl in args: %v", args)
}

func TestSecurityTokenFailureTriggersOCIContextNotification(t *testing.T) {
	tmp := t.TempDir()
	notifyArgsPath := filepath.Join(tmp, "notify-args")
	ociPath := filepath.Join(tmp, "oci")
	ociContextPath := filepath.Join(tmp, "oci-context")
	if err := os.WriteFile(ociPath, []byte(`#!/bin/sh
echo "Security token expired" >&2
exit 1
`), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ociContextPath, []byte(`#!/bin/sh
printf '%s\n' "$@" > "`+notifyArgsPath+`"
exit 0
`), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tmp+":"+os.Getenv("PATH"))

	_, err := (OCIClient{
		Profile:     "OABCS1",
		Region:      "us-chicago-1",
		AuthMethod:  "security_token",
		ContextName: "ord-dev",
	}).CreateSession(TargetDetails{
		BastionID:     "ocid1.bastion.oc1..b1",
		InstanceID:    "ocid1.instance.oc1..i1",
		PrivateIP:     "10.0.0.44",
		TargetUser:    "opc",
		PublicKeyPath: "/tmp/key.pub",
	})
	if err == nil {
		t.Fatal("expected security token error")
	}
	if !strings.Contains(err.Error(), "oci session authenticate --profile-name OABCS1 --region us-chicago-1") {
		t.Fatalf("expected reauth hint with profile-name and region, got: %v", err)
	}
	argsBytes, err := os.ReadFile(notifyArgsPath)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSpace(string(argsBytes)), "\n")
	for _, want := range []string{
		"auth",
		"notify",
		"--profile",
		"OABCS1",
		"--context",
		"ord-dev",
		"--region",
		"us-chicago-1",
		"--native-notify",
	} {
		if !slices.Contains(args, want) {
			t.Fatalf("expected notify args to contain %q, got %v", want, args)
		}
	}
}
