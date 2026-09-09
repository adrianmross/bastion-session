package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWaitForSSHReadyRetriesUntilProbeSucceeds(t *testing.T) {
	// The measured behaviour: the first attempt after ACTIVE fails every time.
	calls := 0
	probe := func(string) bool {
		calls++
		return calls >= 3
	}
	if err := WaitForSSHReady("sess", probe, 5*time.Second, time.Millisecond, nil); err != nil {
		t.Fatalf("expected success once the probe passes, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected to retry until the probe passed, got %d calls", calls)
	}
}

func TestWaitForSSHReadyTimesOut(t *testing.T) {
	err := WaitForSSHReady("sess", func(string) bool { return false }, 20*time.Millisecond, time.Millisecond, nil)
	if err == nil {
		t.Fatal("expected an error when the endpoint never accepts")
	}
	// The message has to point away from "bad key", which is what the raw ssh failure
	// looks like and what sent the original investigation the wrong way.
	if !strings.Contains(err.Error(), "reached ACTIVE") {
		t.Fatalf("error should distinguish ACTIVE from ssh-ready, got: %v", err)
	}
}

func TestWaitForSSHReadyNilProbeIsNoop(t *testing.T) {
	if err := WaitForSSHReady("sess", nil, time.Millisecond, time.Millisecond, nil); err != nil {
		t.Fatalf("a nil probe must not block or fail, got %v", err)
	}
}

// The regression this guards: emitting `IdentityAgent none` for a passphrase-protected
// key makes its only usable copy (the agent's) unreachable, so ssh cannot authenticate.
func TestIdentityHardeningKeepsAgentForPassphraseKey(t *testing.T) {
	dir := t.TempDir()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not available")
	}

	plain := filepath.Join(dir, "plain")
	if out, err := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-f", plain, "-q").CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen (passphraseless): %v: %s", err, out)
	}
	locked := filepath.Join(dir, "locked")
	if out, err := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "hunter2", "-f", locked, "-q").CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen (passphrase): %v: %s", err, out)
	}

	plainLines := strings.Join(identityHardeningLines(plain), "\n")
	if !strings.Contains(plainLines, "IdentitiesOnly yes") {
		t.Error("IdentitiesOnly should always be emitted")
	}
	if !strings.Contains(plainLines, "IdentityAgent none") {
		t.Error("a passphraseless key can safely bypass the agent")
	}

	lockedLines := strings.Join(identityHardeningLines(locked), "\n")
	if !strings.Contains(lockedLines, "IdentitiesOnly yes") {
		t.Error("IdentitiesOnly should always be emitted")
	}
	if strings.Contains(lockedLines, "IdentityAgent none") {
		t.Error("a passphrase-protected key MUST keep the agent, or it cannot authenticate")
	}
}

func TestIdentityHardeningUnknownKeyKeepsAgent(t *testing.T) {
	// Unreadable or absent keys fail safe: keep the agent rather than disabling it.
	for _, path := range []string{"", filepath.Join(t.TempDir(), "missing")} {
		if strings.Contains(strings.Join(identityHardeningLines(path), "\n"), "IdentityAgent none") {
			t.Errorf("unknown key %q must not disable the agent", path)
		}
	}
	junk := filepath.Join(t.TempDir(), "junk")
	if err := os.WriteFile(junk, []byte("not a key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(identityHardeningLines(junk), "\n"), "IdentityAgent none") {
		t.Error("unparseable key must not disable the agent")
	}
}
