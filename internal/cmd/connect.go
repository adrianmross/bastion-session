package cmd

import (
	"fmt"
	"strings"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

func newConnectCmd(opts *rootOptions) *cobra.Command {
	var bastionID string
	var sessionToken string
	var instanceID string
	var privateIP string
	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect using existing session or by creating a new one",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cur, err := loadCurrentSelection(&opts.cfg)
			if err != nil {
				return err
			}
			var session app.BastionSession
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
					BastionID:  bid,
					InstanceID: instanceID,
					PrivateIP:  privateIP,
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
	return cmd
}
