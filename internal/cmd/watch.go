package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

func newWatchCmd(opts *rootOptions) *cobra.Command {
	var interval int
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Continuously refresh bastion session",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var explicit time.Duration
			if interval > 0 {
				explicit = time.Duration(interval) * time.Second
			}
			sleepFor := app.DefaultWatchInterval
			if explicit > 0 {
				sleepFor = explicit
			}
			for {
				s, err := app.RefreshSession(opts.cfg)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to refresh session: %v\n", err)
					sleepFor = app.DefaultWatchInterval
					if explicit > 0 {
						sleepFor = explicit
					}
				} else if explicit == 0 {
					sleepFor = app.AutoRefreshInterval(s)
				}
				fmt.Fprintf(os.Stdout, "Sleeping for %d seconds\n", int(sleepFor.Seconds()))
				time.Sleep(sleepFor)
			}
		},
	}
	cmd.Flags().IntVarP(&interval, "interval", "i", 0, "Refresh interval in seconds")
	return cmd
}
