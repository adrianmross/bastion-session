package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

func newConnectCmd(opts *rootOptions) *cobra.Command {
	var bastionID string
	var sessionToken string
	var instanceID string
	var privateIP string
	var keyOverride string
	var verbose bool
	var waitTimeout time.Duration
	cmd := &cobra.Command{
		Use:   "connect [bastion-ref-or-ocid]",
		Short: "Connect using existing session or by creating a new one",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if keyOverride != "" {
				opts.cfg.SSHPublicKey = keyOverride
			}
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
				if verbose {
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
					WaitTimeout: waitTimeout,
					OnCreated: func(s app.BastionSession) {
						if verbose {
							fmt.Fprintf(cmd.OutOrStdout(), "Created session %s; waiting for ACTIVE...\n", s.ID)
						}
					},
					OnPoll: func(s app.BastionSession) {
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
			fmt.Fprintf(cmd.OutOrStdout(), "Session %s is ACTIVE\n", session.ID)
			fmt.Fprintf(cmd.OutOrStdout(), "Connect with: ssh %s\n", hostAlias)
			return nil
		},
	}
	cmd.Flags().StringVar(&sessionToken, "session", "", "Existing session id/ref to use (no new session created)")
	cmd.Flags().StringVar(&bastionID, "bastion-id", "", "Bastion OCID (defaults to current selected bastion)")
	cmd.Flags().StringVar(&instanceID, "instance-id", "", "Target instance OCID override (otherwise Terraform outputs)")
	cmd.Flags().StringVar(&privateIP, "private-ip", "", "Target private IP override (otherwise Terraform outputs)")
	cmd.Flags().StringVar(&keyOverride, "key", "", "SSH public key path override when creating a new session")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show session creation and lifecycle polling details")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", app.ActiveWaitTimeout, "How long to wait for a newly created session to reach ACTIVE (e.g. 2m, 10m)")
	return cmd
}
