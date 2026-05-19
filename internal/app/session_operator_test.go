package app

import (
	"testing"
	"time"
)

func TestSessionExpiryWarningNearExpiry(t *testing.T) {
	now := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	warning := SessionExpiryWarning(now.Add(5*time.Minute), now)
	if warning == "" {
		t.Fatalf("expected near-expiry warning")
	}
	if SessionExpiryWarning(now.Add(time.Hour), now) != "" {
		t.Fatalf("did not expect warning for long-lived session")
	}
}
