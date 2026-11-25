package app

import (
	"bytes"
	"crypto/ed25519"
	"golang.org/x/crypto/ssh"
	"strings"
	"testing"
	"time"
)

func TestSelectReusableSessionPrefersLongestExpiry(t *testing.T) {
	now := time.Date(2026, 3, 26, 21, 0, 0, 0, time.UTC)
	md := SessionMetadata{
		BastionID:  "ocid1.bastion.oc1..b1",
		InstanceID: "ocid1.instance.oc1..i1",
		PrivateIP:  "10.0.0.12",
	}
	sessions := []SessionInfo{
		{
			ID:             "ocid1.session.oc1..short",
			BastionID:      "ocid1.bastion.oc1..b1",
			LifecycleState: "ACTIVE",
			TargetResource: "ocid1.instance.oc1..i1",
			TargetPrivate:  "10.0.0.12",
			TimeExpires:    now.Add(5 * time.Minute),
			TimeCreated:    now.Add(-10 * time.Minute),
		},
		{
			ID:             "ocid1.session.oc1..long",
			BastionID:      "ocid1.bastion.oc1..b1",
			LifecycleState: "ACTIVE",
			TargetResource: "ocid1.instance.oc1..i1",
			TargetPrivate:  "10.0.0.12",
			TimeExpires:    now.Add(20 * time.Minute),
			TimeCreated:    now.Add(-20 * time.Minute),
		},
	}
	got := reusableSessionCandidates(md, sessions, now, 2*time.Minute)
	if len(got) != 2 || got[0].ID != "ocid1.session.oc1..long" {
		t.Fatalf("expected longest-lived session first, got %v", got)
	}
}

func TestSelectReusableSessionSkipsNearExpiryOrMismatchedTarget(t *testing.T) {
	now := time.Date(2026, 3, 26, 21, 0, 0, 0, time.UTC)
	md := SessionMetadata{
		BastionID:  "ocid1.bastion.oc1..b1",
		InstanceID: "ocid1.instance.oc1..i1",
		PrivateIP:  "10.0.0.12",
	}
	sessions := []SessionInfo{
		{
			ID:             "ocid1.session.oc1..near-expiry",
			BastionID:      "ocid1.bastion.oc1..b1",
			LifecycleState: "ACTIVE",
			TargetResource: "ocid1.instance.oc1..i1",
			TargetPrivate:  "10.0.0.12",
			TimeExpires:    now.Add(30 * time.Second),
		},
		{
			ID:             "ocid1.session.oc1..wrong-instance",
			BastionID:      "ocid1.bastion.oc1..b1",
			LifecycleState: "ACTIVE",
			TargetResource: "ocid1.instance.oc1..other",
			TargetPrivate:  "10.0.0.12",
			TimeExpires:    now.Add(30 * time.Minute),
		},
		{
			ID:             "ocid1.session.oc1..creating",
			BastionID:      "ocid1.bastion.oc1..b1",
			LifecycleState: "CREATING",
			TargetResource: "ocid1.instance.oc1..i1",
			TargetPrivate:  "10.0.0.12",
			TimeExpires:    now.Add(30 * time.Minute),
		},
	}
	if got := reusableSessionCandidates(md, sessions, now, 2*time.Minute); len(got) != 0 {
		t.Fatalf("expected no reusable session, got %v", got)
	}
}

func fixturePublicKey(t *testing.T, fill byte) string {
	t.Helper()
	pub, err := ssh.NewPublicKey(ed25519.PublicKey(bytes.Repeat([]byte{fill}, ed25519.PublicKeySize)))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
}

func TestSessionIsReusableRejectsMismatchedPublicKey(t *testing.T) {
	now := time.Date(2026, 3, 26, 21, 0, 0, 0, time.UTC)
	md := SessionMetadata{
		BastionID:  "ocid1.bastion.oc1..b1",
		InstanceID: "ocid1.instance.oc1..i1",
		PrivateIP:  "10.0.0.12",
	}
	session := BastionSession{
		ID:               "ocid1.session.oc1..wrong-key",
		BastionID:        "ocid1.bastion.oc1..b1",
		TargetResourceID: "ocid1.instance.oc1..i1",
		TargetPrivateIP:  "10.0.0.12",
		PublicKeyContent: fixturePublicKey(t, 1) + " old",
		LifecycleState:   "ACTIVE",
		TimeExpires:      now.Add(20 * time.Minute),
	}
	if sessionIsReusable(md, session, now, 2*time.Minute, fixturePublicKey(t, 2)+" requested") {
		t.Fatal("expected mismatched public key to make session non-reusable")
	}
	session.PublicKeyContent = fixturePublicKey(t, 2) + " different-comment"
	if !sessionIsReusable(md, session, now, 2*time.Minute, fixturePublicKey(t, 2)+" requested") {
		t.Fatal("expected matching public key material to be reusable")
	}
	session.PublicKeyContent = ""
	if sessionIsReusable(md, session, now, 2*time.Minute, fixturePublicKey(t, 2)) {
		t.Fatal("session without reported key cannot prove a match")
	}

}

func TestFindReusableSessionSkipsUnverifiedAndForwardingSessions(t *testing.T) {
	key := fixturePublicKey(t, 2)
	sessions := []BastionSession{
		{ID: "wrong", LifecycleState: "ACTIVE", PublicKeyContent: fixturePublicKey(t, 1)},
		{ID: "unknown", LifecycleState: "ACTIVE"},
		{ID: "forward", LifecycleState: "ACTIVE", SessionType: "PORT_FORWARDING", PublicKeyContent: key},
		{ID: "match", LifecycleState: "ACTIVE", SessionType: "MANAGED_SSH", PublicKeyContent: key + " fixture@example.invalid"},
	}
	var candidates []SessionInfo
	details := map[string]BastionSession{}
	for _, session := range sessions {
		candidates = append(candidates, SessionInfo{ID: session.ID})
		details[session.ID] = session
	}
	get := func(id string) (BastionSession, error) { return details[id], nil }
	got, ok := findReusableSession(candidates, get, SessionMetadata{}, time.Now(), time.Minute, key)
	if !ok || got.ID != "match" {
		t.Fatalf("wanted compatible session, got %q", got.ID)
	}
	if _, ok := findReusableSession(candidates[:3], get, SessionMetadata{}, time.Now(), time.Minute, key); ok {
		t.Fatal("reused a session without a compatible managed SSH identity")
	}
}
