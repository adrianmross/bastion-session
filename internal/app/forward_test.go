package app

import (
	"slices"
	"testing"
)

func TestForwardSSHArgsBindsLoopback(t *testing.T) {
	args := ForwardSSHArgs(Config{Region: "us-phoenix-1", SSHPrivateKey: "/tmp/test-key"},
		"ocid1.bastionsession.oc1..example", 18443, "10.0.1.25", 443)
	if !slices.Contains(args, "127.0.0.1:18443:10.0.1.25:443") {
		t.Fatalf("forward may listen beyond loopback: %v", args)
	}
}
