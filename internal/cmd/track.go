package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

func newTrackCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "track", Short: "Manage tracked bastions"}
	cmd.AddCommand(newTrackRmCmd(opts), newTrackPruneCmd(opts))
	return cmd
}

func newTrackRmCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <ref-or-bastion-ocid> [more...]",
		Short: "Remove one or more tracked bastions",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tracked, err := app.LoadTracked(opts.cfg.TrackedBastionsPath)
			if err != nil {
				return err
			}
			ids := make([]string, 0, len(tracked))
			for _, b := range tracked {
				ids = append(ids, b.ID)
			}
			refs := app.BuildShortRefs(ids, 2)
			removeIDs := make([]string, 0, len(args))
			for _, token := range args {
				t := strings.TrimSpace(token)
				if t == "" {
					continue
				}
				matched := ""
				for _, b := range tracked {
					if b.ID == t || refs[b.ID] == t {
						matched = b.ID
						break
					}
				}
				if matched == "" {
					return fmt.Errorf("tracked bastion %s not found", t)
				}
				removeIDs = append(removeIDs, matched)
			}
			removed, err := app.RemoveTracked(opts.cfg.TrackedBastionsPath, removeIDs...)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %d tracked bastion(s)\n", removed)
			return nil
		},
	}
	return cmd
}

func newTrackPruneCmd(opts *rootOptions) *cobra.Command {
	var removeNotFound bool
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Prune tracked bastions that are deleted or no longer resolvable",
		RunE: func(cmd *cobra.Command, _ []string) error {
			tracked, err := app.LoadTracked(opts.cfg.TrackedBastionsPath)
			if err != nil {
				return err
			}
			if len(tracked) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No tracked bastions to prune")
				return nil
			}
			removeIDs := []string{}
			for _, b := range tracked {
				auth := b.AuthMethod
				if auth == "" && b.Profile == opts.cfg.Profile && b.Region == opts.cfg.Region {
					auth = opts.cfg.AuthMethod
				}
				client := app.OCIClient{Profile: b.Profile, Region: b.Region, AuthMethod: auth}
				live, err := client.GetBastion(b.ID)
				if err != nil {
					msg := strings.ToLower(err.Error())
					if removeNotFound && (strings.Contains(msg, "notauthorizedornotfound") || strings.Contains(msg, "not found")) {
						removeIDs = append(removeIDs, b.ID)
						continue
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "Skipping %s (lookup error): %v\n", b.ID, err)
					continue
				}
				state := strings.ToUpper(strings.TrimSpace(live.LifecycleState))
				if state == "DELETED" || state == "DELETING" {
					removeIDs = append(removeIDs, b.ID)
				}
			}
			removed, err := app.RemoveTracked(opts.cfg.TrackedBastionsPath, removeIDs...)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Pruned %d tracked bastion(s) at %s\n", removed, time.Now().UTC().Format(time.RFC3339))
			return nil
		},
	}
	cmd.Flags().BoolVar(&removeNotFound, "remove-not-found", true, "Remove tracked bastions when OCI returns NotAuthorizedOrNotFound")
	return cmd
}
