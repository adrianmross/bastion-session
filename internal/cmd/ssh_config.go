package cmd

import (
	"fmt"
	"strings"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

func newSSHConfigCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "ssh-config", Short: "Inspect effective SSH configuration"}
	cmd.AddCommand(newSSHConfigShowCmd(opts))
	return cmd
}

func newSSHConfigShowCmd(_ *rootOptions) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "show <host>",
		Short: "Show effective ssh -G configuration for a host",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := app.ReadSSHConfig(args[0])
			if err != nil {
				return err
			}
			switch strings.ToLower(output) {
			case "", "text":
				printSSHConfigText(cmd, cfg)
				return nil
			case "json":
				return printJSONTo(cmd.OutOrStdout(), cfg)
			case "yaml", "yml":
				return printYAMLTo(cmd.OutOrStdout(), cfg)
			default:
				return fmt.Errorf("unsupported output format: %s", output)
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format: text|json|yaml")
	return cmd
}

func printSSHConfigText(cmd *cobra.Command, cfg app.SSHConfig) {
	fmt.Fprintf(cmd.OutOrStdout(), "Host: %s\n", cfg.Host)
	fmt.Fprintf(cmd.OutOrStdout(), "HostName: %s\n", emptyDash(cfg.HostName))
	fmt.Fprintf(cmd.OutOrStdout(), "User: %s\n", emptyDash(cfg.User))
	fmt.Fprintf(cmd.OutOrStdout(), "ProxyJump: %s\n", emptyDash(cfg.ProxyJump))
	fmt.Fprintf(cmd.OutOrStdout(), "IdentityFile: %s\n", emptyDash(cfg.IdentityFile))
}
