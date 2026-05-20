package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

type sessionRow struct {
	Ref       string `json:"ref" yaml:"ref"`
	ID        string `json:"id" yaml:"id"`
	Lifecycle string `json:"lifecycle" yaml:"lifecycle"`
	Created   string `json:"created" yaml:"created"`
	Expires   string `json:"expires" yaml:"expires"`
}

type sessionNewResult struct {
	SessionID        string `json:"session_id" yaml:"session_id"`
	SessionLifecycle string `json:"session_lifecycle" yaml:"session_lifecycle"`
	ExpiresAt        string `json:"expires_at" yaml:"expires_at"`
	TargetPrivateIP  string `json:"target_private_ip,omitempty" yaml:"target_private_ip,omitempty"`
	TargetInstanceID string `json:"target_instance_id,omitempty" yaml:"target_instance_id,omitempty"`
	Profile          string `json:"profile" yaml:"profile"`
	Region           string `json:"region" yaml:"region"`
	Context          string `json:"context,omitempty" yaml:"context,omitempty"`
}

func newSessionCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "session", Short: "Manage bastion sessions"}
	cmd.AddCommand(newSessionListCmd(opts), newSessionUseCmd(opts), newSessionNewCmd(opts), newSessionWaitCmd(opts), newSessionPruneCmd(opts), newSessionRenewCmd(opts))
	return cmd
}

func resolveSessionIDToken(client app.OCIClient, bastionID string, token string) (string, error) {
	sessionID := strings.TrimSpace(token)
	if sessionID == "" {
		return "", fmt.Errorf("empty session token")
	}
	if strings.HasPrefix(sessionID, "ocid1.") {
		return sessionID, nil
	}
	if len(sessionID) <= 8 {
		sessions, err := client.ListSessions(bastionID)
		if err != nil {
			return "", err
		}
		ids := make([]string, 0, len(sessions))
		for _, s := range sessions {
			ids = append(ids, s.ID)
		}
		refs := app.BuildShortRefs(ids, 2)
		for _, s := range sessions {
			if refs[s.ID] == sessionID {
				return s.ID, nil
			}
		}
	}
	return sessionID, nil
}

func newSessionNewCmd(opts *rootOptions) *cobra.Command {
	var bastionID string
	var instanceID string
	var privateIP string
	var keyOverride string
	var output string
	var sessionTTLText string
	cmd := &cobra.Command{
		Use:   "new [bastion-id-or-ref]",
		Short: "Create a new bastion session (explicit create/renew path)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if keyOverride != "" {
				opts.cfg.SSHPublicKey = keyOverride
			}
			sessionTTL, err := parseSessionTTL(sessionTTLText)
			if err != nil {
				return err
			}
			if strings.TrimSpace(bastionID) == "" && len(args) == 1 {
				bastionID = strings.TrimSpace(args[0])
			}
			cur, err := loadCurrentSelection(&opts.cfg)
			if err != nil {
				return err
			}
			bid, err := requireBastionID(cur, bastionID)
			if err != nil {
				return err
			}
			s, err := app.RefreshSessionWithTarget(opts.cfg, app.RefreshOptions{
				BastionID:  bid,
				InstanceID: instanceID,
				PrivateIP:  privateIP,
				SessionTTL: sessionTTL,
			})
			if err != nil {
				return err
			}
			result := sessionNewResult{
				SessionID:        s.ID,
				SessionLifecycle: s.LifecycleState,
				ExpiresAt:        s.TimeExpires.Format(time.RFC3339),
				TargetPrivateIP:  s.TargetPrivateIP,
				TargetInstanceID: s.TargetResourceID,
				Profile:          opts.cfg.Profile,
				Region:           opts.cfg.Region,
			}
			if opts.cfg.ScopedContext != nil {
				result.Context = opts.cfg.ScopedContext.Name
			}
			switch strings.ToLower(output) {
			case "", "text", "table":
				fmt.Fprintf(cmd.OutOrStdout(), "Created session %s\n", s.ID)
				return nil
			case "json":
				return printJSON(result)
			case "yaml", "yml":
				return printYAML(result)
			default:
				return fmt.Errorf("unsupported output format: %s", output)
			}
		},
	}
	cmd.Flags().StringVar(&bastionID, "bastion-id", "", "Bastion OCID (defaults to current selected bastion)")
	cmd.Flags().StringVar(&instanceID, "instance-id", "", "Target instance OCID override (otherwise Terraform outputs)")
	cmd.Flags().StringVar(&privateIP, "private-ip", "", "Target private IP override (otherwise Terraform outputs)")
	cmd.Flags().StringVar(&keyOverride, "key", "", "SSH public key path override for this session creation")
	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format: text|json|yaml")
	cmd.Flags().StringVar(&sessionTTLText, "session-ttl", "", "Requested TTL for newly created sessions as a duration or seconds (e.g. 3h, 10800)")
	return cmd
}

func newSessionListCmd(opts *rootOptions) *cobra.Command {
	var output string
	var bastionID string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List sessions for selected/current bastion",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cur, err := loadCurrentSelection(&opts.cfg)
			if err != nil {
				return err
			}
			bid, err := requireBastionID(cur, bastionID)
			if err != nil {
				return err
			}
			client := app.OCIClient{Profile: opts.cfg.Profile, Region: opts.cfg.Region, AuthMethod: opts.cfg.AuthMethod}
			sessions, err := client.ListSessions(bid)
			if err != nil {
				return err
			}
			ids := make([]string, 0, len(sessions))
			for _, s := range sessions {
				ids = append(ids, s.ID)
			}
			refs := app.BuildShortRefs(ids, 2)
			rows := make([]sessionRow, 0, len(sessions))
			for _, s := range sessions {
				rows = append(rows, sessionRow{Ref: refs[s.ID], ID: s.ID, Lifecycle: s.LifecycleState, Created: s.TimeCreated.Format("2006-01-02T15:04:05Z07:00"), Expires: s.TimeExpires.Format("2006-01-02T15:04:05Z07:00")})
			}
			switch strings.ToLower(output) {
			case "", "table":
				if len(rows) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No sessions found")
					return nil
				}
				for _, r := range rows {
					fmt.Fprintf(cmd.OutOrStdout(), "%s  id=%s  lifecycle=%s  expires=%s\n", r.Ref, r.ID, r.Lifecycle, r.Expires)
				}
				return nil
			case "json":
				return printJSON(rows)
			case "yaml", "yml":
				return printYAML(rows)
			default:
				return fmt.Errorf("unsupported output format: %s", output)
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table|json|yaml")
	cmd.Flags().StringVar(&bastionID, "bastion-id", "", "Bastion OCID (defaults to current selected bastion)")
	return cmd
}

func newSessionUseCmd(opts *rootOptions) *cobra.Command {
	var bastionID string
	cmd := &cobra.Command{
		Use:   "use <session-id-or-ref>",
		Short: "Switch current active session and update SSH config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			token := strings.TrimSpace(args[0])
			cur, err := loadCurrentSelection(&opts.cfg)
			if err != nil {
				return err
			}
			bid, err := requireBastionID(cur, bastionID)
			if err != nil {
				return err
			}
			client := app.OCIClient{Profile: opts.cfg.Profile, Region: opts.cfg.Region, AuthMethod: opts.cfg.AuthMethod}
			sessionID, err := resolveSessionIDToken(client, bid, token)
			if err != nil {
				return err
			}
			s, err := client.GetSession(sessionID)
			if err != nil {
				return err
			}
			if err := app.SaveSession(opts.cfg.SessionStatePath, s); err != nil {
				return err
			}
			if err := app.EnsureSSHInclude(opts.cfg.SSHIncludePath); err != nil {
				return err
			}
			if err := app.UpdateSSHFragment(opts.cfg, s.ID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Switched to session %s\n", s.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&bastionID, "bastion-id", "", "Bastion OCID (defaults to current selected bastion)")
	return cmd
}

func newSessionWaitCmd(opts *rootOptions) *cobra.Command {
	var bastionID string
	var timeout time.Duration
	var poll time.Duration
	var verbose bool
	cmd := &cobra.Command{
		Use:   "wait <session-id-or-ref>",
		Short: "Wait for a session to reach ACTIVE state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			token := strings.TrimSpace(args[0])
			cur, err := loadCurrentSelection(&opts.cfg)
			if err != nil {
				return err
			}
			bid, err := requireBastionID(cur, bastionID)
			if err != nil {
				return err
			}
			client := app.OCIClient{Profile: opts.cfg.Profile, Region: opts.cfg.Region, AuthMethod: opts.cfg.AuthMethod}
			sessionID, err := resolveSessionIDToken(client, bid, token)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Waiting for session %s (timeout=%s poll=%s)\n", sessionID, timeout.String(), poll.String())

			lastPrintedState := ""
			lastPrintedAt := time.Time{}
			s, err := app.WaitForActive(client, sessionID, timeout, poll, func(s app.BastionSession) {
				if !verbose {
					return
				}
				now := time.Now().UTC()
				if strings.EqualFold(s.LifecycleState, lastPrintedState) && now.Sub(lastPrintedAt) < 20*time.Second {
					return
				}
				lastPrintedState = s.LifecycleState
				lastPrintedAt = now
				expires := "-"
				if !s.TimeExpires.IsZero() {
					expires = s.TimeExpires.Format(time.RFC3339)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] session=%s lifecycle=%s expires=%s\n",
					now.Format(time.RFC3339), s.ID, emptyDash(s.LifecycleState), expires)
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Session %s is ACTIVE\n", s.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&bastionID, "bastion-id", "", "Bastion OCID (defaults to current selected bastion)")
	cmd.Flags().DurationVar(&timeout, "timeout", app.ActiveWaitTimeout, "Maximum wait time for ACTIVE (e.g. 2m, 10m, 30m)")
	cmd.Flags().DurationVar(&poll, "poll", app.ActivePollIntervalSeconds, "Polling interval while waiting (e.g. 5s, 15s)")
	cmd.Flags().BoolVar(&verbose, "verbose", true, "Show lifecycle polling output while waiting")
	return cmd
}

func newSessionPruneCmd(opts *rootOptions) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove expired cached session state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			result, err := app.PruneExpiredSession(opts.cfg.SessionStatePath, time.Now())
			if err != nil {
				return err
			}
			switch strings.ToLower(output) {
			case "", "text", "table":
				if result.Pruned {
					fmt.Fprintf(cmd.OutOrStdout(), "Pruned cached session %s\n", result.SessionID)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "No session pruned: %s\n", result.Reason)
				}
				return nil
			case "json":
				return printJSONTo(cmd.OutOrStdout(), result)
			case "yaml", "yml":
				return printYAMLTo(cmd.OutOrStdout(), result)
			default:
				return fmt.Errorf("unsupported output format: %s", output)
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format: text|json|yaml")
	return cmd
}

func newSessionRenewCmd(opts *rootOptions) *cobra.Command {
	var bastionID string
	var instanceID string
	var privateIP string
	var keyOverride string
	var targetIdentityFile string
	var output string
	var sessionTTLText string
	var waitTimeout time.Duration
	cmd := &cobra.Command{
		Use:   "renew <ssh-host>",
		Short: "Ensure/refresh an active session for a tracked SSH host",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionTTL, err := parseSessionTTL(sessionTTLText)
			if err != nil {
				return err
			}
			result, err := runEnsureHost(&opts.cfg, args[0], ensureRunOptions{
				BastionID:          bastionID,
				InstanceID:         instanceID,
				PrivateIP:          privateIP,
				KeyOverride:        keyOverride,
				TargetIdentityFile: targetIdentityFile,
				SessionTTL:         sessionTTL,
				WaitTimeout:        waitTimeout,
				TargetUserExplicit: opts.targetUser != "",
			})
			if err != nil {
				return err
			}
			switch strings.ToLower(output) {
			case "", "text", "table":
				fmt.Fprintf(cmd.OutOrStdout(), "Renewed: %s\n", result.ConnectCommand)
				fmt.Fprintf(cmd.OutOrStdout(), "Session: %s\n", result.SessionID)
				fmt.Fprintf(cmd.OutOrStdout(), "Expires: %s\n", result.ExpiresAt)
				return nil
			case "json":
				return printJSONTo(cmd.OutOrStdout(), result)
			case "yaml", "yml":
				return printYAMLTo(cmd.OutOrStdout(), result)
			default:
				return fmt.Errorf("unsupported output format: %s", output)
			}
		},
	}
	cmd.Flags().StringVar(&bastionID, "bastion-id", "", "Bastion OCID (defaults to current selected bastion)")
	cmd.Flags().StringVar(&instanceID, "instance-id", "", "Target instance OCID override (otherwise tracked target/Terraform outputs)")
	cmd.Flags().StringVar(&privateIP, "private-ip", "", "Target private IP override (otherwise tracked target/Terraform outputs)")
	cmd.Flags().StringVar(&keyOverride, "key", "", "SSH public key path override when creating a new session")
	cmd.Flags().StringVar(&targetIdentityFile, "target-identity-file", "", "SSH private key for the target VM host alias")
	cmd.Flags().StringVar(&targetIdentityFile, "identity-file", "", "SSH private key for the target VM host alias")
	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format: text|json|yaml")
	cmd.Flags().StringVar(&sessionTTLText, "session-ttl", "", "Requested TTL for newly created sessions as a duration or seconds (e.g. 3h, 10800)")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", app.ActiveWaitTimeout, "How long to wait for a newly created session to reach ACTIVE (e.g. 2m, 10m)")
	return cmd
}
