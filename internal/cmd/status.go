package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

func newStatusCmd(opts *rootOptions) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show session status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st, err := app.SessionStatus(opts.cfg)
			if err != nil {
				return err
			}
			switch strings.ToLower(output) {
			case "table", "":
				fmt.Fprintln(os.Stdout, "Bastion Session Status")
				fmt.Fprintf(os.Stdout, "Session ID: %s\n", st.SessionID)
				fmt.Fprintf(os.Stdout, "Lifecycle:  %s\n", st.Lifecycle)
				fmt.Fprintf(os.Stdout, "Expires:    %s\n", st.Expires)
				fmt.Fprintf(os.Stdout, "Expires In: %s\n", st.ExpiresIn)
				fmt.Fprintf(os.Stdout, "Profile:    %s\n", st.Profile)
				fmt.Fprintf(os.Stdout, "Region:     %s\n", st.Region)
				if st.Context != "" {
					fmt.Fprintf(os.Stdout, "Context:    %s\n", st.Context)
				}
				return nil
			case "json":
				return printJSON(st)
			case "yaml", "yml":
				return printYAML(st)
			default:
				return fmt.Errorf("unsupported output format: %s", output)
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table|json|yaml")
	return cmd
}
