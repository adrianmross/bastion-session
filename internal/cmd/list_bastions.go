package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

type bastionRow struct {
	Ref           string `json:"ref" yaml:"ref"`
	ID            string `json:"id" yaml:"id"`
	Name          string `json:"name" yaml:"name"`
	Lifecycle     string `json:"lifecycle" yaml:"lifecycle"`
	CompartmentID string `json:"compartment_id" yaml:"compartment_id"`
	Region        string `json:"region" yaml:"region"`
	Profile       string `json:"profile" yaml:"profile"`
	SSHPublicKey  string `json:"ssh_public_key,omitempty" yaml:"ssh_public_key,omitempty"`
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
				ids := make([]string, 0, len(scoped))
				for _, b := range scoped {
					ids = append(ids, b.ID)
				}
				refs := app.BuildShortRefs(ids, 2)
				for _, b := range scoped {
					rows = append(rows, bastionRow{
						Ref:           refs[b.ID],
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
				ids := make([]string, 0, len(tracked))
				for _, b := range tracked {
					ids = append(ids, b.ID)
				}
				refs := app.BuildShortRefs(ids, 2)
				for _, b := range tracked {
					rows = append(rows, bastionRow{
						Ref:           refs[b.ID],
						ID:            b.ID,
						Name:          b.Name,
						Lifecycle:     "",
						CompartmentID: b.CompartmentID,
						Region:        b.Region,
						Profile:       b.Profile,
						SSHPublicKey:  b.SSHPublicKey,
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
					ref := r.Ref
					if ref == "" {
						ref = "-"
					}
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
					key := r.SSHPublicKey
					if key == "" {
						key = "-"
					}
					fmt.Fprintf(os.Stdout, "%s  id=%s  name=%s  lifecycle=%s  key=%s  context=%s  source=%s\n", ref, r.ID, name, life, key, ctx, r.Source)
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
