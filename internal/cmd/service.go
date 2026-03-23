package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newServiceCmd(_ *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Generate/install background service configs",
	}
	cmd.AddCommand(newServiceLaunchdCmd(), newServiceSystemdCmd())
	return cmd
}

func newServiceLaunchdCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "launchd",
		Short: "Generate/install launchd configuration",
	}
	cmd.AddCommand(newServiceLaunchdGenerateCmd(), newServiceLaunchdInstallCmd())
	return cmd
}

func newServiceLaunchdGenerateCmd() *cobra.Command {
	var (
		label      string
		binaryPath string
		outPath    string
		stdoutPath string
		stderrPath string
		interval   int
	)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate launchd plist for bastion-session watch",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runtime.GOOS != "darwin" {
				return fmt.Errorf("launchd generation is only supported on macOS")
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			if interval <= 0 {
				return fmt.Errorf("--interval must be > 0")
			}
			if binaryPath == "" {
				binaryPath, err = resolveBinaryPath("bastion-session")
				if err != nil {
					return err
				}
			}
			if stdoutPath == "" {
				stdoutPath = filepath.Join(home, ".bastion-session", "watch.out.log")
			}
			if stderrPath == "" {
				stderrPath = filepath.Join(home, ".bastion-session", "watch.err.log")
			}
			if outPath == "" {
				outPath = filepath.Join(home, "Library", "LaunchAgents", label+".plist")
			}
			plist := renderLaunchdPlist(label, binaryPath, interval, stdoutPath, stderrPath)
			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(outPath, []byte(plist), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", outPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Load with:\nlaunchctl unload %s 2>/dev/null || true\nlaunchctl load %s\nlaunchctl start %s\n", outPath, outPath, label)
			return nil
		},
	}

	cmd.Flags().StringVar(&label, "label", "com.remote.bastion-session", "launchd label")
	cmd.Flags().StringVar(&binaryPath, "binary", "", "Absolute path to bastion-session binary")
	cmd.Flags().StringVar(&outPath, "out", "", "Output plist path (default ~/Library/LaunchAgents/<label>.plist)")
	cmd.Flags().StringVar(&stdoutPath, "stdout-log", "", "stdout log path")
	cmd.Flags().StringVar(&stderrPath, "stderr-log", "", "stderr log path")
	cmd.Flags().IntVar(&interval, "interval", 300, "Watch interval in seconds")
	return cmd
}

func newServiceLaunchdInstallCmd() *cobra.Command {
	var (
		label      string
		binaryPath string
		outPath    string
		stdoutPath string
		stderrPath string
		interval   int
		loadNow    bool
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Generate and optionally load launchd plist",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runtime.GOOS != "darwin" {
				return fmt.Errorf("launchd install is only supported on macOS")
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			if interval <= 0 {
				return fmt.Errorf("--interval must be > 0")
			}
			if binaryPath == "" {
				binaryPath, err = resolveBinaryPath("bastion-session")
				if err != nil {
					return err
				}
			}
			if stdoutPath == "" {
				stdoutPath = filepath.Join(home, ".bastion-session", "watch.out.log")
			}
			if stderrPath == "" {
				stderrPath = filepath.Join(home, ".bastion-session", "watch.err.log")
			}
			if outPath == "" {
				outPath = filepath.Join(home, "Library", "LaunchAgents", label+".plist")
			}

			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(outPath, []byte(renderLaunchdPlist(label, binaryPath, interval, stdoutPath, stderrPath)), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", outPath)

			if !loadNow {
				fmt.Fprintf(cmd.OutOrStdout(), "Load with:\nlaunchctl unload %s 2>/dev/null || true\nlaunchctl load %s\nlaunchctl start %s\n", outPath, outPath, label)
				return nil
			}

			_ = exec.Command("launchctl", "unload", outPath).Run()
			if out, err := exec.Command("launchctl", "load", outPath).CombinedOutput(); err != nil {
				return fmt.Errorf("launchctl load failed: %v: %s", err, strings.TrimSpace(string(out)))
			}
			if out, err := exec.Command("launchctl", "start", label).CombinedOutput(); err != nil {
				return fmt.Errorf("launchctl start failed: %v: %s", err, strings.TrimSpace(string(out)))
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Loaded and started launchd agent.")
			return nil
		},
	}

	cmd.Flags().StringVar(&label, "label", "com.remote.bastion-session", "launchd label")
	cmd.Flags().StringVar(&binaryPath, "binary", "", "Absolute path to bastion-session binary")
	cmd.Flags().StringVar(&outPath, "out", "", "Output plist path (default ~/Library/LaunchAgents/<label>.plist)")
	cmd.Flags().StringVar(&stdoutPath, "stdout-log", "", "stdout log path")
	cmd.Flags().StringVar(&stderrPath, "stderr-log", "", "stderr log path")
	cmd.Flags().IntVar(&interval, "interval", 300, "Watch interval in seconds")
	cmd.Flags().BoolVar(&loadNow, "load", true, "Load and start launchd agent after writing plist")
	return cmd
}

func newServiceSystemdCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "systemd",
		Short: "Generate/install systemd --user unit",
	}
	cmd.AddCommand(newServiceSystemdGenerateCmd(), newServiceSystemdInstallCmd())
	return cmd
}

func newServiceSystemdGenerateCmd() *cobra.Command {
	var (
		serviceName string
		binaryPath  string
		outPath     string
		interval    int
	)
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a systemd --user unit for bastion-session watch",
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			if interval <= 0 {
				return fmt.Errorf("--interval must be > 0")
			}
			if binaryPath == "" {
				binaryPath, err = resolveBinaryPath("bastion-session")
				if err != nil {
					return err
				}
			}
			if outPath == "" {
				outPath = filepath.Join(home, ".config", "systemd", "user", serviceName)
			}

			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(outPath, []byte(renderSystemdUnit(binaryPath, interval)), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", outPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Enable with:\nsystemctl --user daemon-reload\nsystemctl --user enable --now %s\n", strings.TrimSuffix(filepath.Base(outPath), ".service"))
			return nil
		},
	}

	cmd.Flags().StringVar(&serviceName, "service-name", "bastion-session.service", "systemd service unit filename")
	cmd.Flags().StringVar(&binaryPath, "binary", "", "Absolute path to bastion-session binary")
	cmd.Flags().StringVar(&outPath, "out", "", "Output service path (default ~/.config/systemd/user/<service-name>)")
	cmd.Flags().IntVar(&interval, "interval", 300, "Watch interval in seconds")
	return cmd
}

func newServiceSystemdInstallCmd() *cobra.Command {
	var (
		serviceName string
		binaryPath  string
		outPath     string
		interval    int
		enableNow   bool
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Generate and optionally enable/start a systemd --user unit",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runtime.GOOS != "linux" {
				return fmt.Errorf("systemd install is only supported on Linux")
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			if interval <= 0 {
				return fmt.Errorf("--interval must be > 0")
			}
			if binaryPath == "" {
				binaryPath, err = resolveBinaryPath("bastion-session")
				if err != nil {
					return err
				}
			}
			if outPath == "" {
				outPath = filepath.Join(home, ".config", "systemd", "user", serviceName)
			}

			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(outPath, []byte(renderSystemdUnit(binaryPath, interval)), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %s\n", outPath)

			unit := strings.TrimSuffix(filepath.Base(outPath), ".service")
			if !enableNow {
				fmt.Fprintf(cmd.OutOrStdout(), "Enable with:\nsystemctl --user daemon-reload\nsystemctl --user enable --now %s\n", unit)
				return nil
			}

			if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
				return fmt.Errorf("systemctl daemon-reload failed: %v: %s", err, strings.TrimSpace(string(out)))
			}
			if out, err := exec.Command("systemctl", "--user", "enable", "--now", unit).CombinedOutput(); err != nil {
				return fmt.Errorf("systemctl enable --now failed: %v: %s", err, strings.TrimSpace(string(out)))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Enabled and started %s\n", unit)
			return nil
		},
	}

	cmd.Flags().StringVar(&serviceName, "service-name", "bastion-session.service", "systemd service unit filename")
	cmd.Flags().StringVar(&binaryPath, "binary", "", "Absolute path to bastion-session binary")
	cmd.Flags().StringVar(&outPath, "out", "", "Output service path (default ~/.config/systemd/user/<service-name>)")
	cmd.Flags().IntVar(&interval, "interval", 300, "Watch interval in seconds")
	cmd.Flags().BoolVar(&enableNow, "enable", true, "Run systemctl --user daemon-reload and enable --now")
	return cmd
}

func resolveBinaryPath(bin string) (string, error) {
	if path, err := exec.LookPath(bin); err == nil {
		return path, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not resolve %s binary path; pass --binary", bin)
	}
	return exe, nil
}

func renderLaunchdPlist(label, binaryPath string, interval int, stdoutPath, stderrPath string) string {
	args := []string{
		xmlEscape(binaryPath),
		"watch",
		"--interval",
		strconv.Itoa(interval),
	}
	argXML := make([]string, 0, len(args))
	for _, a := range args {
		argXML = append(argXML, fmt.Sprintf("      <string>%s</string>", xmlEscape(a)))
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
%s
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>ProcessType</key>
    <string>Background</string>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
  </dict>
</plist>
`, xmlEscape(label), strings.Join(argXML, "\n"), xmlEscape(stdoutPath), xmlEscape(stderrPath))
}

func renderSystemdUnit(binaryPath string, interval int) string {
	return fmt.Sprintf(`[Unit]
Description=OCI Bastion Session Watcher
After=network-online.target

[Service]
Type=simple
ExecStart=%s watch --interval %d
Restart=on-failure
RestartSec=30

[Install]
WantedBy=default.target
`, shellEscapeForSystemd(binaryPath), interval)
}

func shellEscapeForSystemd(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n\"'$") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func xmlEscape(s string) string {
	repl := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return repl.Replace(s)
}
