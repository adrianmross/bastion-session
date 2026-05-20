package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

type connectResult struct {
	Ready            bool   `json:"ready" yaml:"ready"`
	SSHHost          string `json:"ssh_host" yaml:"ssh_host"`
	ConnectCommand   string `json:"connect_command" yaml:"connect_command"`
	SessionID        string `json:"session_id" yaml:"session_id"`
	SessionLifecycle string `json:"session_lifecycle" yaml:"session_lifecycle"`
	ExpiresAt        string `json:"expires_at" yaml:"expires_at"`
	TargetPrivateIP  string `json:"target_private_ip,omitempty" yaml:"target_private_ip,omitempty"`
	TargetInstanceID string `json:"target_instance_id,omitempty" yaml:"target_instance_id,omitempty"`
	Profile          string `json:"profile" yaml:"profile"`
	Region           string `json:"region" yaml:"region"`
	Context          string `json:"context,omitempty" yaml:"context,omitempty"`
}

func newConnectCmd(opts *rootOptions) *cobra.Command {
	var bastionID string
	var sessionToken string
	var instanceID string
	var privateIP string
	var keyOverride string
	var output string
	var verbose bool
	var sessionTTLText string
	var waitTimeout time.Duration
	cmd := &cobra.Command{
		Use:   "connect [bastion-ref-or-ocid]",
		Short: "Connect using existing session or by creating a new one",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if keyOverride != "" {
				opts.cfg.SSHPublicKey = keyOverride
			}
			sessionTTL, err := parseSessionTTL(sessionTTLText)
			if err != nil {
				return err
			}
			structuredOutput := strings.EqualFold(output, "json") || strings.EqualFold(output, "yaml") || strings.EqualFold(output, "yml")
			if len(args) == 1 {
				if strings.TrimSpace(sessionToken) != "" {
					return fmt.Errorf("positional bastion token cannot be used with --session")
				}
				if strings.TrimSpace(bastionID) != "" {
					return fmt.Errorf("positional bastion token cannot be used with --bastion-id")
				}
				resolved, err := resolveBastionIDToken(&opts.cfg, args[0])
				if err != nil {
					return err
				}
				bastionID = resolved
			}
			cur, err := loadCurrentSelection(&opts.cfg)
			if err != nil {
				return err
			}
			// `use` selection is authoritative for identity settings in connect flows.
			applyCurrentSelectionIdentity(&opts.cfg, cur)
			var session app.BastionSession
			lastPrintedState := ""
			lastPrintedAt := time.Time{}
			if strings.TrimSpace(sessionToken) != "" {
				bid, err := requireBastionID(cur, bastionID)
				if err != nil {
					return err
				}
				client := app.OCIClient{Profile: opts.cfg.Profile, Region: opts.cfg.Region, AuthMethod: opts.cfg.AuthMethod}
				sessionID := strings.TrimSpace(sessionToken)
				if len(sessionID) <= 8 {
					sessions, err := client.ListSessions(bid)
					if err != nil {
						return err
					}
					ids := make([]string, 0, len(sessions))
					for _, s := range sessions {
						ids = append(ids, s.ID)
					}
					refs := app.BuildShortRefs(ids, 2)
					for _, s := range sessions {
						if refs[s.ID] == sessionID {
							sessionID = s.ID
							break
						}
					}
				}
				if verbose && !structuredOutput {
					fmt.Fprintf(cmd.OutOrStdout(), "Fetching existing session %s for bastion %s\n", sessionID, bid)
				}
				session, err = client.GetSession(sessionID)
				if err != nil {
					return err
				}
			} else {
				bid, err := requireBastionID(cur, bastionID)
				if err != nil {
					return err
				}
				session, err = app.RefreshSessionWithTarget(opts.cfg, app.RefreshOptions{
					BastionID:   bid,
					InstanceID:  instanceID,
					PrivateIP:   privateIP,
					SessionTTL:  sessionTTL,
					WaitTimeout: waitTimeout,
					OnCreated: func(s app.BastionSession) {
						if verbose && !structuredOutput {
							fmt.Fprintf(cmd.OutOrStdout(), "Created session %s; waiting for ACTIVE...\n", s.ID)
						}
					},
					OnReused: func(s app.BastionSession) {
						if verbose && !structuredOutput {
							expires := "-"
							if !s.TimeExpires.IsZero() {
								expires = s.TimeExpires.Format(time.RFC3339)
							}
							fmt.Fprintf(cmd.OutOrStdout(), "Reusing ACTIVE session %s (expires=%s)\n", s.ID, expires)
						}
					},
					OnPoll: func(s app.BastionSession) {
						if !verbose || structuredOutput {
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
					},
				})
				if err != nil {
					return err
				}
			}
			if err := app.SaveSession(opts.cfg.SessionStatePath, session); err != nil {
				return err
			}
			if err := app.EnsureSSHInclude(opts.cfg.SSHIncludePath); err != nil {
				return err
			}
			if err := app.UpdateSSHFragment(opts.cfg, session.ID); err != nil {
				return err
			}
			hostAlias := opts.cfg.Profile + "-bastion"
			result := connectResult{
				Ready:            strings.EqualFold(session.LifecycleState, "ACTIVE"),
				SSHHost:          hostAlias,
				ConnectCommand:   "ssh " + hostAlias,
				SessionID:        session.ID,
				SessionLifecycle: session.LifecycleState,
				ExpiresAt:        session.TimeExpires.Format(time.RFC3339),
				TargetPrivateIP:  session.TargetPrivateIP,
				TargetInstanceID: session.TargetResourceID,
				Profile:          opts.cfg.Profile,
				Region:           opts.cfg.Region,
			}
			if opts.cfg.ScopedContext != nil {
				result.Context = opts.cfg.ScopedContext.Name
			}
			switch strings.ToLower(output) {
			case "", "text", "table":
				fmt.Fprintf(cmd.OutOrStdout(), "Session %s is ACTIVE\n", session.ID)
				fmt.Fprintf(cmd.OutOrStdout(), "Connect with: ssh %s\n", hostAlias)
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
	cmd.Flags().StringVar(&sessionToken, "session", "", "Existing session id/ref to use (no new session created)")
	cmd.Flags().StringVar(&bastionID, "bastion-id", "", "Bastion OCID (defaults to current selected bastion)")
	cmd.Flags().StringVar(&instanceID, "instance-id", "", "Target instance OCID override (otherwise Terraform outputs)")
	cmd.Flags().StringVar(&privateIP, "private-ip", "", "Target private IP override (otherwise Terraform outputs)")
	cmd.Flags().StringVar(&keyOverride, "key", "", "SSH public key path override when creating a new session")
	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format: text|json|yaml")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show session creation and lifecycle polling details")
	cmd.Flags().StringVar(&sessionTTLText, "session-ttl", "", "Requested TTL for newly created sessions as a duration or seconds (e.g. 3h, 10800)")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", app.ActiveWaitTimeout, "How long to wait for a newly created session to reach ACTIVE (e.g. 2m, 10m)")
	return cmd
}
