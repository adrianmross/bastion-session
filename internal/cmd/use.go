package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

func newUseCmd(opts *rootOptions) *cobra.Command {
	var source string
	var name string
	var profile string
	var region string
	var compartmentID string
	var contextName string

	cmd := &cobra.Command{
		Use:   "use <bastion-ocid>",
		Short: "Select a current bastion from scoped/tracked sources or explicit details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			token := strings.TrimSpace(args[0])
			source = strings.ToLower(strings.TrimSpace(source))
			if source == "" {
				source = "tracked"
			}
			var cur app.CurrentBastion
			found := false

			switch source {
			case "tracked":
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
					if b.ID == token || refs[b.ID] == token {
						cur = app.CurrentBastion{
							ID:            b.ID,
							Name:          b.Name,
							CompartmentID: b.CompartmentID,
							Region:        b.Region,
							Profile:       b.Profile,
							AuthMethod:    b.AuthMethod,
							ContextName:   b.ContextName,
							Source:        "tracked",
							SelectedAt:    time.Now().UTC(),
						}
						found = true
						break
					}
				}
			case "scoped":
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
					if b.ID == token || refs[b.ID] == token {
						cur = app.CurrentBastion{
							ID:            b.ID,
							Name:          b.Name,
							CompartmentID: b.CompartmentID,
							Region:        b.Region,
							Profile:       b.Profile,
							AuthMethod:    opts.cfg.AuthMethod,
							ContextName:   b.ScopeContext,
							Source:        "scoped",
							SelectedAt:    time.Now().UTC(),
						}
						found = true
						break
					}
				}
			default:
				return fmt.Errorf("unsupported source: %s", source)
			}

			if !found {
				// Manual explicit selection path when full details are provided.
				if strings.TrimSpace(profile) == "" || strings.TrimSpace(region) == "" || strings.TrimSpace(compartmentID) == "" {
					return fmt.Errorf("bastion %s not found in %s; provide --profile, --region, and --compartment-id for explicit use", token, source)
				}
				cur = app.CurrentBastion{
					ID:            token,
					Name:          name,
					CompartmentID: compartmentID,
					Region:        region,
					Profile:       profile,
					AuthMethod:    opts.cfg.AuthMethod,
					ContextName:   contextName,
					Source:        source,
					SelectedAt:    time.Now().UTC(),
				}
			}

			if err := app.SaveCurrent(opts.cfg.CurrentStatePath, cur); err != nil {
				return err
			}
			_ = app.UpsertTracked(opts.cfg.TrackedBastionsPath, app.TrackedBastion{
				ID:            cur.ID,
				Name:          cur.Name,
				CompartmentID: cur.CompartmentID,
				Region:        cur.Region,
				Profile:       cur.Profile,
				AuthMethod:    cur.AuthMethod,
				ContextName:   cur.ContextName,
				LastSeenAt:    time.Now().UTC(),
			})
			_, err := fmt.Fprintf(os.Stdout, "Using bastion %s (source=%s profile=%s region=%s compartment=%s)\n", cur.ID, cur.Source, cur.Profile, cur.Region, cur.CompartmentID)
			return err
		},
	}
	cmd.Flags().StringVar(&source, "source", "tracked", "Selection source: tracked|scoped")
	cmd.Flags().StringVar(&name, "name", "", "Bastion display name for explicit/manual selection")
	cmd.Flags().StringVarP(&profile, "profile", "p", "", "Profile override for explicit/manual selection")
	cmd.Flags().StringVarP(&region, "region", "r", "", "Region override for explicit/manual selection")
	cmd.Flags().StringVar(&compartmentID, "compartment-id", "", "Compartment OCID for explicit/manual selection")
	cmd.Flags().StringVar(&contextName, "context", "", "Context label for explicit/manual selection")
	return cmd
}
