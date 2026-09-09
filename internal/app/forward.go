package app

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ForwardSSHArgs builds the ssh invocation for a port-forwarding session.
//
// Kept in one place so the readiness probe and the real tunnel cannot drift apart -- a
// probe that succeeds under different options than the tunnel uses proves nothing.
func ForwardSSHArgs(cfg Config, sessionID string, localPort int, targetIP string, targetPort int) []string {
	args := []string{}
	if key := resolvePrivateKey(cfg.SSHPrivateKey, cfg.SSHPublicKey); key != "" {
		args = append(args, "-i", key)
	}
	args = append(args,
		"-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=accept-new",
		// Without this ssh reports success while the forward is dead, and the failure
		// only shows up later as a connection refused from whatever uses the tunnel.
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ConnectTimeout=10",
		"-N", "-L", fmt.Sprintf("%d:%s:%d", localPort, targetIP, targetPort),
		"-p", "22",
		fmt.Sprintf("%s@host.bastion.%s.oci.oraclecloud.com", sessionID, cfg.Region),
	)
	return args
}

// SSHForwardProbe returns a probe that establishes the forward briefly and reports whether
// the bastion accepted it.
//
// It runs the real ssh binary with the same options as the real tunnel, because the thing
// being tested is precisely whether ssh can authenticate yet. Two earlier probe designs
// gave confident false answers and are worth not repeating:
//
//   - `ssh -T` is always denied on a port-forwarding session, which forbids a shell, so it
//     reports "not ready" forever.
//   - Grepping for the absence of "denied" also matches an ssh that died instantly for an
//     unrelated reason, most easily a local port already in use.
//
// So this asserts positively: the process must still be alive with the forward bound.
func SSHForwardProbe(cfg Config, targetIP string, targetPort int, probeLocalPort int) SSHProbe {
	return func(sessionID string) bool {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		args := ForwardSSHArgs(cfg, sessionID, probeLocalPort, targetIP, targetPort)
		cmd := exec.CommandContext(ctx, "ssh", args...)
		var sink strings.Builder
		cmd.Stdout = &sink
		cmd.Stderr = &sink
		if err := cmd.Start(); err != nil {
			return false
		}
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()

		select {
		case <-done:
			// Exiting on its own means it failed: -N never terminates on success.
			return false
		case <-time.After(3 * time.Second):
			// Still running with ExitOnForwardFailure set: the forward is bound.
			_ = cmd.Process.Kill()
			<-done
			return true
		}
	}
}
