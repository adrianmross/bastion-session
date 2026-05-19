package cmd

import (
	"fmt"
	"strings"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

type explainResult struct {
	Host       string           `json:"host" yaml:"host"`
	SSHConfig  *app.SSHConfig   `json:"ssh_config,omitempty" yaml:"ssh_config,omitempty"`
	Target     *targetRow       `json:"target,omitempty" yaml:"target,omitempty"`
	Session    doctorSession    `json:"session" yaml:"session"`
	Bastion    doctorCurrent    `json:"bastion" yaml:"bastion"`
	Config     doctorConfig     `json:"config" yaml:"config"`
	SSHInclude doctorSSHInclude `json:"ssh_include" yaml:"ssh_include"`
	Issues     []doctorIssue    `json:"issues,omitempty" yaml:"issues,omitempty"`
}

func newExplainCmd(opts *rootOptions) *cobra.Command {
	var output string
	var cachedOnly bool
	cmd := &cobra.Command{
		Use:   "explain <host>",
		Short: "Explain the SSH path, session, target, and bastion context for a host",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := strings.TrimSpace(args[0])
			report := buildDoctorReport(opts.cfg, host, doctorOptions{Live: !cachedOnly})
			result := explainResult{
				Host:       host,
				SSHConfig:  report.SSHConfig,
				Target:     report.Target,
				Session:    report.Session,
				Bastion:    report.Current,
				Config:     report.Config,
				SSHInclude: report.SSHInclude,
				Issues:     report.Issues,
			}
			switch strings.ToLower(output) {
			case "", "text", "table":
				printExplainText(cmd, result)
			case "json":
				if err := printJSONTo(cmd.OutOrStdout(), result); err != nil {
					return err
				}
			case "yaml", "yml":
				if err := printYAMLTo(cmd.OutOrStdout(), result); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported output format: %s", output)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format: text|json|yaml")
	cmd.Flags().BoolVar(&cachedOnly, "cached", false, "Use cached local state only; do not call live OCI APIs")
	cmd.Flags().BoolVar(&cachedOnly, "no-live", false, "Use cached local state only; do not call live OCI APIs")
	return cmd
}

func printExplainText(cmd *cobra.Command, result explainResult) {
	fmt.Fprintf(cmd.OutOrStdout(), "Host: %s\n", result.Host)
	if result.SSHConfig != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "SSH HostName: %s\n", emptyDash(result.SSHConfig.HostName))
		fmt.Fprintf(cmd.OutOrStdout(), "SSH User: %s\n", emptyDash(result.SSHConfig.User))
		fmt.Fprintf(cmd.OutOrStdout(), "SSH ProxyJump: %s\n", emptyDash(result.SSHConfig.ProxyJump))
		fmt.Fprintf(cmd.OutOrStdout(), "SSH IdentityFile: %s\n", emptyDash(result.SSHConfig.IdentityFile))
	}
	if result.Target != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Tracked Target: %s private_ip=%s instance=%s bastion=%s\n",
			result.Target.Name, emptyDash(result.Target.PrivateIP), emptyDash(result.Target.InstanceID), emptyDash(result.Target.BastionID))
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Tracked Target: unavailable")
	}
	if result.Session.Cached != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Cached Session: %s lifecycle=%s expires=%s\n",
			result.Session.Cached.ID, emptyDash(result.Session.Cached.Lifecycle), emptyDash(result.Session.Cached.Expires))
	}
	if result.Session.Live != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "Live Session: %s lifecycle=%s expires=%s\n",
			result.Session.Live.ID, emptyDash(result.Session.Live.Lifecycle), emptyDash(result.Session.Live.Expires))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Current Bastion: %s\n", availability(result.Bastion.Available))
	if result.Bastion.ID != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Current Bastion ID: %s\n", result.Bastion.ID)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Profile: %s\n", emptyDash(result.Config.Profile))
	fmt.Fprintf(cmd.OutOrStdout(), "Region: %s\n", emptyDash(result.Config.Region))
	for _, issue := range result.Issues {
		fmt.Fprintf(cmd.OutOrStdout(), "Issue: %s severity=%s %s\n", issue.Code, issue.Severity, issue.Message)
	}
}
