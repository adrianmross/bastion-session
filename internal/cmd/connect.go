package cmd

import (
	"fmt"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

func newConnectCmd(opts *rootOptions) *cobra.Command {
	var bastionID string
	var instanceID string
	var privateIP string
	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Create/refresh a session for current bastion and print SSH connect hint",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cur, err := loadCurrentSelection(&opts.cfg)
			if err != nil {
				return err
			}
			bid, err := requireBastionID(cur, bastionID)
			if err != nil {
				return err
			}
			session, err := app.RefreshSessionWithTarget(opts.cfg, app.RefreshOptions{
				BastionID:  bid,
				InstanceID: instanceID,
				PrivateIP:  privateIP,
			})
			if err != nil {
				return err
			}
			hostAlias := opts.cfg.Profile + "-bastion"
			fmt.Fprintf(cmd.OutOrStdout(), "Session %s is ACTIVE\n", session.ID)
			fmt.Fprintf(cmd.OutOrStdout(), "Connect with: ssh %s\n", hostAlias)
			return nil
		},
	}
	cmd.Flags().StringVar(&bastionID, "bastion-id", "", "Bastion OCID (defaults to current selected bastion)")
	cmd.Flags().StringVar(&instanceID, "instance-id", "", "Target instance OCID override (otherwise Terraform outputs)")
	cmd.Flags().StringVar(&privateIP, "private-ip", "", "Target private IP override (otherwise Terraform outputs)")
	return cmd
}
