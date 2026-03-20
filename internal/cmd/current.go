package cmd

import (
	"fmt"
	"strings"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

func newCurrentCmd(opts *rootOptions) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "current",
		Short: "Show currently selected bastion",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cur, err := app.LoadCurrent(opts.cfg.CurrentStatePath)
			if err != nil {
				return err
			}
			if cur == nil {
				return fmt.Errorf("no current bastion selected")
			}
			switch strings.ToLower(output) {
			case "", "table":
				ref := app.BuildShortRefs([]string{cur.ID}, 2)[cur.ID]
				fmt.Fprintf(cmd.OutOrStdout(), "id: %s\n", cur.ID)
				fmt.Fprintf(cmd.OutOrStdout(), "ref: %s\n", emptyDash(ref))
				fmt.Fprintf(cmd.OutOrStdout(), "name: %s\n", emptyDash(cur.Name))
				fmt.Fprintf(cmd.OutOrStdout(), "source: %s\n", emptyDash(cur.Source))
				fmt.Fprintf(cmd.OutOrStdout(), "profile: %s\n", emptyDash(cur.Profile))
				fmt.Fprintf(cmd.OutOrStdout(), "region: %s\n", emptyDash(cur.Region))
				fmt.Fprintf(cmd.OutOrStdout(), "compartment: %s\n", emptyDash(cur.CompartmentID))
				fmt.Fprintf(cmd.OutOrStdout(), "context: %s\n", emptyDash(cur.ContextName))
				fmt.Fprintf(cmd.OutOrStdout(), "selected_at: %s\n", cur.SelectedAt.Format("2006-01-02T15:04:05Z07:00"))
				return nil
			case "json":
				return printJSON(cur)
			case "yaml", "yml":
				return printYAML(cur)
			default:
				return fmt.Errorf("unsupported output format: %s", output)
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table|json|yaml")
	return cmd
}
