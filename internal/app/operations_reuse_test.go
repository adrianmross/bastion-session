package app

import (
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
	got, ok := selectReusableSession(md, sessions, now, 2*time.Minute)
	if !ok {
		t.Fatalf("expected reusable session")
	}
	if got != "ocid1.session.oc1..long" {
		t.Fatalf("unexpected reusable session id: %s", got)
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
	if got, ok := selectReusableSession(md, sessions, now, 2*time.Minute); ok {
		t.Fatalf("expected no reusable session, got %s", got)
	}
}
