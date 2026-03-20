package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

type bastionRow struct {
	ID            string `json:"id" yaml:"id"`
	Name          string `json:"name" yaml:"name"`
	Lifecycle     string `json:"lifecycle" yaml:"lifecycle"`
	CompartmentID string `json:"compartment_id" yaml:"compartment_id"`
	Region        string `json:"region" yaml:"region"`
	Profile       string `json:"profile" yaml:"profile"`
	Context       string `json:"context,omitempty" yaml:"context,omitempty"`
	Source        string `json:"source" yaml:"source"`
}

func newListCmd(opts *rootOptions) *cobra.Command {
	var output string
	var source string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List bastions scoped by oci-context or from tracked store",
		RunE: func(cmd *cobra.Command, _ []string) error {
			source = strings.ToLower(source)
			rows := []bastionRow{}
			if source == "scoped" || source == "all" {
				scoped, err := app.ListScopedBastions(opts.cfg)
				if err != nil {
					return err
				}
				for _, b := range scoped {
					rows = append(rows, bastionRow{
						ID:            b.ID,
						Name:          b.Name,
						Lifecycle:     b.LifecycleState,
						CompartmentID: b.CompartmentID,
						Region:        b.Region,
						Profile:       b.Profile,
						Context:       b.ScopeContext,
						Source:        "scoped",
					})
				}
			}
			if source == "tracked" || source == "all" {
				tracked, err := app.LoadTracked(opts.cfg.TrackedBastionsPath)
				if err != nil {
					return err
				}
				for _, b := range tracked {
					rows = append(rows, bastionRow{
						ID:            b.ID,
						Name:          b.Name,
						Lifecycle:     "",
						CompartmentID: b.CompartmentID,
						Region:        b.Region,
						Profile:       b.Profile,
						Context:       b.ContextName,
						Source:        "tracked",
					})
				}
			}
			switch strings.ToLower(output) {
			case "table", "":
				if len(rows) == 0 {
					fmt.Fprintln(os.Stdout, "No bastions found")
					return nil
				}
				for _, r := range rows {
					name := r.Name
					if name == "" {
						name = "-"
					}
					ctx := r.Context
					if ctx == "" {
						ctx = "-"
					}
					life := r.Lifecycle
					if life == "" {
						life = "-"
					}
					fmt.Fprintf(os.Stdout, "%s  name=%s  lifecycle=%s  context=%s  source=%s\n", r.ID, name, life, ctx, r.Source)
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
	cmd.Flags().StringVar(&source, "source", "scoped", "Data source: scoped|tracked|all")
	return cmd
}
