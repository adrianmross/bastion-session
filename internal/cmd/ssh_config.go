package cmd

import (
	"fmt"
	"strings"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

func newSSHConfigCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "ssh-config", Short: "Inspect effective SSH configuration"}
	cmd.AddCommand(newSSHConfigShowCmd(opts), newSSHConfigAuditCmd(opts))
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

func newSSHConfigAuditCmd(opts *rootOptions) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "audit <host>",
		Short: "Scan SSH config files for matching Host blocks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			audit := app.AuditSSHConfig(args[0], opts.cfg.SSHIncludePath)
			switch strings.ToLower(output) {
			case "", "text", "table":
				fmt.Fprintf(cmd.OutOrStdout(), "Host: %s\n", audit.Host)
				fmt.Fprintf(cmd.OutOrStdout(), "Competing: %t\n", audit.Competing)
				for _, match := range audit.Matches {
					fmt.Fprintf(cmd.OutOrStdout(), "%s:%d Host %s exact=%t\n", match.Path, match.Line, strings.Join(match.Patterns, " "), match.ExactMatch)
				}
				for _, scanErr := range audit.ScanErrors {
					fmt.Fprintf(cmd.OutOrStdout(), "Scan Error: %s\n", scanErr)
				}
				if audit.Warning != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "Warning: %s\n", audit.Warning)
				}
				return nil
			case "json":
				return printJSONTo(cmd.OutOrStdout(), audit)
			case "yaml", "yml":
				return printYAMLTo(cmd.OutOrStdout(), audit)
			default:
				return fmt.Errorf("unsupported output format: %s", output)
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format: text|json|yaml")
	return cmd
}
