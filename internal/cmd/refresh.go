package cmd

import (
	"fmt"
	"os"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

func newRefreshCmd(opts *rootOptions) *cobra.Command {
	var keyOverride string
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Create or refresh a bastion session",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if keyOverride != "" {
				opts.cfg.SSHPublicKey = keyOverride
			}
			cur, err := loadCurrentSelection(&opts.cfg)
			if err != nil {
				return err
			}
			refreshOpts := app.RefreshOptions{}
			if cur != nil && cur.ID != "" {
				refreshOpts.BastionID = cur.ID
			}
			session, err := app.RefreshSessionWithTarget(opts.cfg, refreshOpts)
			if err != nil {
				return err
			}
			ctxInfo := ""
			if opts.cfg.ScopedContext != nil {
				ctxInfo = fmt.Sprintf(" [context=%s]", opts.cfg.ScopedContext.Name)
			}
			_, err = fmt.Fprintf(os.Stdout, "Created session %s%s, expires at %s\n", session.ID, ctxInfo, session.TimeExpires.Format("2006-01-02T15:04:05Z07:00"))
			return err
		},
	}
	cmd.Flags().StringVar(&keyOverride, "key", "", "SSH public key path override for this refresh")
	return cmd
}
