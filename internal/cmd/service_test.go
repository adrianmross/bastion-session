package cmd

import (
	"strings"
	"testing"
)

func TestRenderLaunchdPlist(t *testing.T) {
	plist := renderLaunchdPlist(
		"com.example.bastion-session",
		"/usr/local/bin/bastion-session",
		300,
		"/Users/me/.bastion-session/watch.out.log",
		"/Users/me/.bastion-session/watch.err.log",
	)
	for _, want := range []string{
		"<string>com.example.bastion-session</string>",
		"<string>/usr/local/bin/bastion-session</string>",
		"<string>watch</string>",
		"<string>--interval</string>",
		"<string>300</string>",
		"<string>/Users/me/.bastion-session/watch.out.log</string>",
		"<string>/Users/me/.bastion-session/watch.err.log</string>",
	} {
		if !strings.Contains(plist, want) {
			t.Fatalf("expected launchd plist to contain %q", want)
		}
	}
}

func TestRenderSystemdUnit(t *testing.T) {
	unit := renderSystemdUnit("/opt/homebrew/bin/bastion-session", 600)
	for _, want := range []string{
		"Description=OCI Bastion Session Watcher",
		"ExecStart=/opt/homebrew/bin/bastion-session watch --interval 600",
		"Restart=on-failure",
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("expected systemd unit to contain %q", want)
		}
	}
}
