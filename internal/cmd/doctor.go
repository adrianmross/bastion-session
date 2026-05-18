package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

type doctorReport struct {
	Config      doctorConfig     `json:"config" yaml:"config"`
	Current     doctorCurrent    `json:"current_bastion" yaml:"current_bastion"`
	Target      *targetRow       `json:"tracked_target,omitempty" yaml:"tracked_target,omitempty"`
	TargetError string           `json:"tracked_target_error,omitempty" yaml:"tracked_target_error,omitempty"`
	Session     doctorSession    `json:"session" yaml:"session"`
	SSHInclude  doctorSSHInclude `json:"ssh_include" yaml:"ssh_include"`
	SSHConfig   *app.SSHConfig   `json:"ssh_config,omitempty" yaml:"ssh_config,omitempty"`
}

type doctorConfig struct {
	Profile             string `json:"profile" yaml:"profile"`
	Region              string `json:"region" yaml:"region"`
	AuthMethod          string `json:"auth_method,omitempty" yaml:"auth_method,omitempty"`
	Context             string `json:"context,omitempty" yaml:"context,omitempty"`
	ContextConfigPath   string `json:"context_config_path,omitempty" yaml:"context_config_path,omitempty"`
	ContextScopeEnabled bool   `json:"context_scope_enabled" yaml:"context_scope_enabled"`
}

type doctorCurrent struct {
	Available    bool   `json:"available" yaml:"available"`
	ID           string `json:"id,omitempty" yaml:"id,omitempty"`
	Name         string `json:"name,omitempty" yaml:"name,omitempty"`
	Source       string `json:"source,omitempty" yaml:"source,omitempty"`
	Profile      string `json:"profile,omitempty" yaml:"profile,omitempty"`
	Region       string `json:"region,omitempty" yaml:"region,omitempty"`
	Context      string `json:"context,omitempty" yaml:"context,omitempty"`
	SelectedAt   string `json:"selected_at,omitempty" yaml:"selected_at,omitempty"`
	Error        string `json:"error,omitempty" yaml:"error,omitempty"`
	TrackedCount int    `json:"tracked_count" yaml:"tracked_count"`
	TrackedError string `json:"tracked_error,omitempty" yaml:"tracked_error,omitempty"`
	CurrentPath  string `json:"current_path" yaml:"current_path"`
	TrackedPath  string `json:"tracked_path" yaml:"tracked_path"`
}

type doctorSession struct {
	Cached      *doctorSessionInfo `json:"cached,omitempty" yaml:"cached,omitempty"`
	CachedError string             `json:"cached_error,omitempty" yaml:"cached_error,omitempty"`
	Live        *doctorSessionInfo `json:"live,omitempty" yaml:"live,omitempty"`
	LiveError   string             `json:"live_error,omitempty" yaml:"live_error,omitempty"`
	StatePath   string             `json:"state_path" yaml:"state_path"`
}

type doctorSessionInfo struct {
	ID               string `json:"id" yaml:"id"`
	BastionID        string `json:"bastion_id,omitempty" yaml:"bastion_id,omitempty"`
	TargetResourceID string `json:"target_resource_id,omitempty" yaml:"target_resource_id,omitempty"`
	TargetPrivateIP  string `json:"target_private_ip,omitempty" yaml:"target_private_ip,omitempty"`
	Lifecycle        string `json:"lifecycle" yaml:"lifecycle"`
	Created          string `json:"created,omitempty" yaml:"created,omitempty"`
	Expires          string `json:"expires,omitempty" yaml:"expires,omitempty"`
	ExpiresIn        string `json:"expires_in,omitempty" yaml:"expires_in,omitempty"`
}

type doctorSSHInclude struct {
	Path    string `json:"path" yaml:"path"`
	Exists  bool   `json:"exists" yaml:"exists"`
	IsDir   bool   `json:"is_dir,omitempty" yaml:"is_dir,omitempty"`
	Error   string `json:"error,omitempty" yaml:"error,omitempty"`
	Present bool   `json:"present" yaml:"present"`
}

func newDoctorCmd(opts *rootOptions) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "doctor [host]",
		Short: "Diagnose local bastion-session and SSH configuration",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := ""
			if len(args) > 0 {
				host = strings.TrimSpace(args[0])
			}
			report := buildDoctorReport(opts.cfg, host)
			switch strings.ToLower(output) {
			case "", "text":
				printDoctorText(cmd, report, host)
				return nil
			case "json":
				return printJSONTo(cmd.OutOrStdout(), report)
			case "yaml", "yml":
				return printYAMLTo(cmd.OutOrStdout(), report)
			default:
				return fmt.Errorf("unsupported output format: %s", output)
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format: text|json|yaml")
	return cmd
}

func buildDoctorReport(cfg app.Config, host string) doctorReport {
	report := doctorReport{
		Config: doctorConfig{
			Profile:             cfg.Profile,
			Region:              cfg.Region,
			AuthMethod:          cfg.AuthMethod,
			ContextConfigPath:   cfg.OCIContextConfigPath,
			ContextScopeEnabled: cfg.ContextScopeEnabled,
		},
		Current: doctorCurrent{
			CurrentPath: cfg.CurrentStatePath,
			TrackedPath: cfg.TrackedBastionsPath,
		},
		Session: doctorSession{StatePath: cfg.SessionStatePath},
		SSHInclude: doctorSSHInclude{
			Path: cfg.SSHIncludePath,
		},
	}
	if cfg.ScopedContext != nil {
		report.Config.Context = cfg.ScopedContext.Name
	}

	if current, err := app.LoadCurrent(cfg.CurrentStatePath); err != nil {
		report.Current.Error = err.Error()
	} else if current != nil {
		report.Current.Available = true
		report.Current.ID = current.ID
		report.Current.Name = current.Name
		report.Current.Source = current.Source
		report.Current.Profile = current.Profile
		report.Current.Region = current.Region
		report.Current.Context = current.ContextName
		report.Current.SelectedAt = formatTime(current.SelectedAt)
	}
	if tracked, err := app.LoadTracked(cfg.TrackedBastionsPath); err != nil {
		report.Current.TrackedError = err.Error()
	} else {
		report.Current.TrackedCount = len(tracked)
	}

	if cached, err := app.LoadSession(cfg.SessionStatePath); err != nil {
		report.Session.CachedError = err.Error()
	} else if cached != nil {
		report.Session.Cached = doctorSessionFromApp(*cached)
		client := app.OCIClient{Profile: cfg.Profile, Region: cfg.Region, AuthMethod: cfg.AuthMethod}
		if live, err := client.GetSession(cached.ID); err != nil {
			report.Session.LiveError = err.Error()
		} else {
			report.Session.Live = doctorSessionFromApp(live)
		}
	}

	if st, err := os.Stat(cfg.SSHIncludePath); err != nil {
		report.SSHInclude.Error = err.Error()
		report.SSHInclude.Exists = false
		report.SSHInclude.Present = false
	} else {
		report.SSHInclude.Exists = true
		report.SSHInclude.Present = true
		report.SSHInclude.IsDir = st.IsDir()
	}

	if host != "" {
		if target, err := app.FindTrackedTarget(cfg.TrackedTargetsPath, host); err == nil && target != nil {
			row := targetRows([]app.TrackedTarget{*target})[0]
			report.Target = &row
		} else if err != nil {
			report.TargetError = err.Error()
		}
		sshCfg, _ := app.ReadSSHConfig(host)
		report.SSHConfig = &sshCfg
	}
	return report
}

func doctorSessionFromApp(s app.BastionSession) *doctorSessionInfo {
	info := &doctorSessionInfo{
		ID:               s.ID,
		BastionID:        s.BastionID,
		TargetResourceID: s.TargetResourceID,
		TargetPrivateIP:  s.TargetPrivateIP,
		Lifecycle:        s.LifecycleState,
		Created:          formatTime(s.TimeCreated),
		Expires:          formatTime(s.TimeExpires),
	}
	if !s.TimeExpires.IsZero() {
		if s.TimeExpires.After(time.Now()) {
			info.ExpiresIn = time.Until(s.TimeExpires).Round(time.Second).String()
		} else {
			info.ExpiresIn = "expired " + time.Since(s.TimeExpires).Round(time.Second).String() + " ago"
		}
	}
	return info
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func printDoctorText(cmd *cobra.Command, report doctorReport, host string) {
	fmt.Fprintln(cmd.OutOrStdout(), "Bastion Session Doctor")
	fmt.Fprintf(cmd.OutOrStdout(), "Profile: %s\n", emptyDash(report.Config.Profile))
	fmt.Fprintf(cmd.OutOrStdout(), "Region: %s\n", emptyDash(report.Config.Region))
	fmt.Fprintf(cmd.OutOrStdout(), "Auth Method: %s\n", emptyDash(report.Config.AuthMethod))
	fmt.Fprintf(cmd.OutOrStdout(), "Context: %s\n", emptyDash(report.Config.Context))
	fmt.Fprintf(cmd.OutOrStdout(), "Current Bastion: %s\n", availability(report.Current.Available))
	if report.Current.ID != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Current Bastion ID: %s\n", report.Current.ID)
	}
	if report.Current.Error != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Current Error: %s\n", report.Current.Error)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Tracked Bastions: %d\n", report.Current.TrackedCount)
	if report.Current.TrackedError != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Tracked Bastions Error: %s\n", report.Current.TrackedError)
	}
	if report.Session.Cached != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Cached Session: %s (%s)\n", report.Session.Cached.ID, emptyDash(report.Session.Cached.Lifecycle))
	} else if report.Session.CachedError != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Cached Session Error: %s\n", report.Session.CachedError)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Cached Session: unavailable")
	}
	if report.Session.Live != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Live Session: %s (%s)\n", report.Session.Live.ID, emptyDash(report.Session.Live.Lifecycle))
	} else if report.Session.LiveError != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Live Session Error: %s\n", report.Session.LiveError)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "SSH Include: %s (%s)\n", report.SSHInclude.Path, availability(report.SSHInclude.Exists && !report.SSHInclude.IsDir))
	if report.SSHInclude.Error != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "SSH Include Error: %s\n", report.SSHInclude.Error)
	}
	if host != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Host: %s\n", host)
		if report.Target != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Tracked Target: %s private_ip=%s bastion=%s\n", report.Target.Name, emptyDash(report.Target.PrivateIP), emptyDash(report.Target.BastionID))
		} else if report.TargetError != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Tracked Target Error: %s\n", report.TargetError)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Tracked Target: unavailable")
		}
		if report.SSHConfig != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "SSH HostName: %s\n", emptyDash(report.SSHConfig.HostName))
			fmt.Fprintf(cmd.OutOrStdout(), "SSH User: %s\n", emptyDash(report.SSHConfig.User))
			fmt.Fprintf(cmd.OutOrStdout(), "SSH ProxyJump: %s\n", emptyDash(report.SSHConfig.ProxyJump))
			fmt.Fprintf(cmd.OutOrStdout(), "SSH IdentityFile: %s\n", emptyDash(report.SSHConfig.IdentityFile))
			if report.SSHConfig.Error != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "SSH Config Error: %s\n", report.SSHConfig.Error)
			}
		}
	}
}

func availability(ok bool) string {
	if ok {
		return "available"
	}
	return "unavailable"
}
