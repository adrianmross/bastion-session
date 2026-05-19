package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type pathsResult struct {
	SSHIncludePath       string `json:"ssh_include_path" yaml:"ssh_include_path"`
	SessionStatePath     string `json:"session_state_path" yaml:"session_state_path"`
	CurrentStatePath     string `json:"current_state_path" yaml:"current_state_path"`
	TrackedBastionsPath  string `json:"tracked_bastions_path" yaml:"tracked_bastions_path"`
	TrackedTargetsPath   string `json:"tracked_targets_path" yaml:"tracked_targets_path"`
	TerraformOutputsPath string `json:"terraform_outputs_path,omitempty" yaml:"terraform_outputs_path,omitempty"`
	OCIContextConfigPath string `json:"oci_context_config_path,omitempty" yaml:"oci_context_config_path,omitempty"`
}

func newPathsCmd(opts *rootOptions) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "paths",
		Short: "Show bastion-session local state paths",
		RunE: func(cmd *cobra.Command, _ []string) error {
			result := pathsResult{
				SSHIncludePath:       opts.cfg.SSHIncludePath,
				SessionStatePath:     opts.cfg.SessionStatePath,
				CurrentStatePath:     opts.cfg.CurrentStatePath,
				TrackedBastionsPath:  opts.cfg.TrackedBastionsPath,
				TrackedTargetsPath:   opts.cfg.TrackedTargetsPath,
				TerraformOutputsPath: opts.cfg.TerraformOutputsPath,
				OCIContextConfigPath: opts.cfg.OCIContextConfigPath,
			}
			switch strings.ToLower(output) {
			case "", "text", "table":
				fmt.Fprintf(cmd.OutOrStdout(), "SSH Include: %s\n", result.SSHIncludePath)
				fmt.Fprintf(cmd.OutOrStdout(), "Session State: %s\n", result.SessionStatePath)
				fmt.Fprintf(cmd.OutOrStdout(), "Current Bastion: %s\n", result.CurrentStatePath)
				fmt.Fprintf(cmd.OutOrStdout(), "Tracked Bastions: %s\n", result.TrackedBastionsPath)
				fmt.Fprintf(cmd.OutOrStdout(), "Tracked Targets: %s\n", result.TrackedTargetsPath)
				if result.TerraformOutputsPath != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "Terraform Outputs: %s\n", result.TerraformOutputsPath)
				}
				if result.OCIContextConfigPath != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "OCI Context Config: %s\n", result.OCIContextConfigPath)
				}
				return nil
			case "json":
				return printJSONTo(cmd.OutOrStdout(), result)
			case "yaml", "yml":
				return printYAMLTo(cmd.OutOrStdout(), result)
			default:
				return fmt.Errorf("unsupported output format: %s", output)
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format: text|json|yaml")
	return cmd
}
