package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

type ensureResult struct {
	Ready            bool   `json:"ready" yaml:"ready"`
	SSHHost          string `json:"ssh_host" yaml:"ssh_host"`
	ConnectCommand   string `json:"connect_command" yaml:"connect_command"`
	ProxyJump        string `json:"proxy_jump" yaml:"proxy_jump"`
	SessionID        string `json:"session_id" yaml:"session_id"`
	SessionLifecycle string `json:"session_lifecycle" yaml:"session_lifecycle"`
	ExpiresAt        string `json:"expires_at" yaml:"expires_at"`
	TargetPrivateIP  string `json:"target_private_ip" yaml:"target_private_ip"`
	TargetInstanceID string `json:"target_instance_id" yaml:"target_instance_id"`
	Profile          string `json:"profile" yaml:"profile"`
	Region           string `json:"region" yaml:"region"`
	Context          string `json:"context,omitempty" yaml:"context,omitempty"`
}

func newEnsureCmd(opts *rootOptions) *cobra.Command {
	var bastionID string
	var instanceID string
	var privateIP string
	var keyOverride string
	var targetIdentityFile string
	var output string
	var waitTimeout time.Duration
	cmd := &cobra.Command{
		Use:   "ensure <ssh-host>",
		Short: "Ensure an active bastion session and VM-facing SSH host alias",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sshHost := strings.TrimSpace(args[0])
			if sshHost == "" {
				return fmt.Errorf("ssh host alias is required")
			}
			if keyOverride != "" {
				opts.cfg.SSHPublicKey = keyOverride
			}
			trackedTarget, err := app.FindTrackedTarget(opts.cfg.TrackedTargetsPath, sshHost)
			if err != nil {
				return err
			}
			applyTrackedTargetForEnsure(&opts.cfg, trackedTarget, &bastionID, &instanceID, &privateIP, &targetIdentityFile, opts.targetUser != "")
			cur, err := loadCurrentSelection(&opts.cfg)
			if err != nil {
				return err
			}
			applyCurrentSelectionIdentity(&opts.cfg, cur)
			bid, err := requireBastionID(cur, bastionID)
			if err != nil {
				return err
			}
			session, err := app.RefreshSessionWithTarget(opts.cfg, app.RefreshOptions{
				BastionID:   bid,
				InstanceID:  instanceID,
				PrivateIP:   privateIP,
				WaitTimeout: waitTimeout,
			})
			if err != nil {
				return err
			}
			targetIP := strings.TrimSpace(privateIP)
			if targetIP == "" {
				targetIP = strings.TrimSpace(session.TargetPrivateIP)
			}
			targetInstanceID := strings.TrimSpace(instanceID)
			if targetInstanceID == "" {
				targetInstanceID = strings.TrimSpace(session.TargetResourceID)
			}
			if targetIP == "" {
				return fmt.Errorf("unable to determine target private IP for %s; pass --private-ip", sshHost)
			}
			if err := app.EnsureSSHInclude(opts.cfg.SSHIncludePath); err != nil {
				return err
			}
			proxyJump := opts.cfg.Profile + "-bastion"
			if err := app.UpdateSSHFragmentWithTarget(opts.cfg, session.ID, app.TargetSSHHost{
				Alias:        sshHost,
				HostName:     targetIP,
				User:         opts.cfg.TargetUser,
				IdentityFile: targetIdentityFile,
				ProxyJump:    proxyJump,
			}); err != nil {
				return err
			}
			result := ensureResult{
				Ready:            strings.EqualFold(session.LifecycleState, "ACTIVE"),
				SSHHost:          sshHost,
				ConnectCommand:   "ssh " + sshHost,
				ProxyJump:        proxyJump,
				SessionID:        session.ID,
				SessionLifecycle: session.LifecycleState,
				ExpiresAt:        session.TimeExpires.Format(time.RFC3339),
				TargetPrivateIP:  targetIP,
				TargetInstanceID: targetInstanceID,
				Profile:          opts.cfg.Profile,
				Region:           opts.cfg.Region,
			}
			if opts.cfg.ScopedContext != nil {
				result.Context = opts.cfg.ScopedContext.Name
			}
			switch strings.ToLower(output) {
			case "", "text", "table":
				fmt.Fprintf(cmd.OutOrStdout(), "Ready: %s\n", result.ConnectCommand)
				fmt.Fprintf(cmd.OutOrStdout(), "Session: %s\n", result.SessionID)
				fmt.Fprintf(cmd.OutOrStdout(), "Target: %s (%s)\n", result.SSHHost, result.TargetPrivateIP)
				fmt.Fprintf(cmd.OutOrStdout(), "ProxyJump: %s\n", result.ProxyJump)
				fmt.Fprintf(cmd.OutOrStdout(), "Expires: %s\n", result.ExpiresAt)
				return nil
			case "json":
				return printJSON(result)
			case "yaml", "yml":
				return printYAML(result)
			default:
				return fmt.Errorf("unsupported output format: %s", output)
			}
		},
	}
	cmd.Flags().StringVar(&bastionID, "bastion-id", "", "Bastion OCID (defaults to current selected bastion)")
	cmd.Flags().StringVar(&instanceID, "instance-id", "", "Target instance OCID override (otherwise Terraform outputs)")
	cmd.Flags().StringVar(&privateIP, "private-ip", "", "Target private IP override (otherwise Terraform outputs)")
	cmd.Flags().StringVar(&keyOverride, "key", "", "SSH public key path override when creating a new session")
	cmd.Flags().StringVar(&targetIdentityFile, "target-identity-file", "", "SSH private key for the target VM host alias")
	cmd.Flags().StringVar(&targetIdentityFile, "identity-file", "", "SSH private key for the target VM host alias")
	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format: text|json|yaml")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", app.ActiveWaitTimeout, "How long to wait for a newly created session to reach ACTIVE (e.g. 2m, 10m)")
	return cmd
}

func applyTrackedTargetForEnsure(cfg *app.Config, target *app.TrackedTarget, bastionID, instanceID, privateIP, identityFile *string, targetUserExplicit bool) {
	if cfg == nil || target == nil {
		return
	}
	if bastionID != nil && strings.TrimSpace(*bastionID) == "" && strings.TrimSpace(target.BastionID) != "" {
		*bastionID = strings.TrimSpace(target.BastionID)
	}
	if instanceID != nil && strings.TrimSpace(*instanceID) == "" && strings.TrimSpace(target.InstanceID) != "" {
		*instanceID = strings.TrimSpace(target.InstanceID)
	}
	if privateIP != nil && strings.TrimSpace(*privateIP) == "" && strings.TrimSpace(target.PrivateIP) != "" {
		*privateIP = strings.TrimSpace(target.PrivateIP)
	}
	if identityFile != nil && strings.TrimSpace(*identityFile) == "" && strings.TrimSpace(target.IdentityFile) != "" {
		*identityFile = strings.TrimSpace(target.IdentityFile)
	}
	if !targetUserExplicit && strings.TrimSpace(target.User) != "" {
		cfg.TargetUser = strings.TrimSpace(target.User)
	}
	if strings.TrimSpace(cfg.TerraformOutputsPath) == "" && strings.TrimSpace(target.TerraformOutputsPath) != "" {
		cfg.TerraformOutputsPath = strings.TrimSpace(target.TerraformOutputsPath)
	}
}
