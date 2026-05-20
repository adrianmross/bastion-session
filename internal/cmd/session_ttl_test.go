package cmd

import (
	"testing"
	"time"
)

func TestParseSessionTTLAcceptsDurationOrSeconds(t *testing.T) {
	tests := map[string]time.Duration{
		"3h":    3 * time.Hour,
		"10800": 3 * time.Hour,
		"45m":   45 * time.Minute,
		"":      0,
	}
	for input, want := range tests {
		got, err := parseSessionTTL(input)
		if err != nil {
			t.Fatalf("parseSessionTTL(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("parseSessionTTL(%q)=%s, want %s", input, got, want)
		}
	}
}

func TestParseSessionTTLRejectsInvalidValues(t *testing.T) {
	for _, input := range []string{"0", "-1s", "500ms", "not-a-duration"} {
		if got, err := parseSessionTTL(input); err == nil {
			t.Fatalf("parseSessionTTL(%q)=%s, want error", input, got)
		}
	}
}
