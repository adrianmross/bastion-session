package cmd

import (
	"fmt"
	"strings"

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

func newSessionCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "session", Short: "Manage bastion sessions"}
	cmd.AddCommand(newSessionListCmd(opts), newSessionUseCmd(opts), newSessionNewCmd(opts))
	return cmd
}

func newSessionNewCmd(opts *rootOptions) *cobra.Command {
	var bastionID string
	var instanceID string
	var privateIP string
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new bastion session (explicit create/renew path)",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created session %s\n", s.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&bastionID, "bastion-id", "", "Bastion OCID (defaults to current selected bastion)")
	cmd.Flags().StringVar(&instanceID, "instance-id", "", "Target instance OCID override (otherwise Terraform outputs)")
	cmd.Flags().StringVar(&privateIP, "private-ip", "", "Target private IP override (otherwise Terraform outputs)")
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
			sessionID := token
			if len(token) <= 8 {
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
					if refs[s.ID] == token {
						sessionID = s.ID
						break
					}
				}
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
