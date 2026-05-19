package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

type targetRow struct {
	Name                 string `json:"name" yaml:"name"`
	InstanceID           string `json:"instance_id" yaml:"instance_id"`
	PrivateIP            string `json:"private_ip" yaml:"private_ip"`
	User                 string `json:"user,omitempty" yaml:"user,omitempty"`
	IdentityFile         string `json:"identity_file,omitempty" yaml:"identity_file,omitempty"`
	BastionID            string `json:"bastion_id,omitempty" yaml:"bastion_id,omitempty"`
	TerraformOutputsPath string `json:"terraform_outputs,omitempty" yaml:"terraform_outputs,omitempty"`
	LastSeenAt           string `json:"last_seen_at" yaml:"last_seen_at"`
}

func newTargetCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "target", Short: "Manage tracked VM targets"}
	cmd.AddCommand(newTargetTrackCmd(opts), newTargetImportCmd(opts), newTargetReconcileCmd(opts), newTargetListCmd(opts), newTargetShowCmd(opts), newTargetRmCmd(opts))
	return cmd
}

func newTargetTrackCmd(opts *rootOptions) *cobra.Command {
	var instanceID string
	var privateIP string
	var user string
	var identityFile string
	var bastionID string
	var terraformOutputs string
	cmd := &cobra.Command{
		Use:   "track <name>",
		Short: "Track a VM target for ensure",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			target := app.TrackedTarget{
				Name:                 name,
				InstanceID:           strings.TrimSpace(instanceID),
				PrivateIP:            strings.TrimSpace(privateIP),
				User:                 strings.TrimSpace(user),
				IdentityFile:         strings.TrimSpace(identityFile),
				BastionID:            strings.TrimSpace(bastionID),
				TerraformOutputsPath: strings.TrimSpace(terraformOutputs),
				LastSeenAt:           time.Now().UTC(),
			}
			if target.TerraformOutputsPath == "" {
				target.TerraformOutputsPath = strings.TrimSpace(opts.cfg.TerraformOutputsPath)
			}
			if target.TerraformOutputsPath != "" && (target.InstanceID == "" || target.PrivateIP == "" || target.BastionID == "") {
				outputs, err := app.ReadOutputs(target.TerraformOutputsPath)
				if err != nil {
					return err
				}
				if target.InstanceID == "" {
					target.InstanceID = outputString(outputs, "instance_id")
				}
				if target.PrivateIP == "" {
					target.PrivateIP = outputString(outputs, "private_ip")
				}
				if target.BastionID == "" {
					target.BastionID = outputString(outputs, "bastion_id")
				}
			}
			if target.InstanceID == "" {
				return fmt.Errorf("--instance-id is required unless supplied by --terraform-outputs")
			}
			if target.PrivateIP == "" {
				return fmt.Errorf("--private-ip is required unless supplied by --terraform-outputs")
			}
			if err := app.UpsertTrackedTarget(opts.cfg.TrackedTargetsPath, target); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Tracked target %s\n", target.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&instanceID, "instance-id", "", "Target instance OCID")
	cmd.Flags().StringVar(&privateIP, "private-ip", "", "Target private IP")
	cmd.Flags().StringVar(&user, "user", "", "Target OS user override")
	cmd.Flags().StringVar(&identityFile, "identity-file", "", "SSH private key for the target VM host alias")
	cmd.Flags().StringVar(&bastionID, "bastion-id", "", "Bastion OCID for this target")
	cmd.Flags().StringVar(&terraformOutputs, "terraform-outputs", "", "Path to Terraform state/outputs file for this target")
	return cmd
}

func newTargetImportCmd(opts *rootOptions) *cobra.Command {
	var user string
	var identityFile string
	var bastionID string
	var terraformOutputs string
	cmd := &cobra.Command{
		Use:   "import <name>",
		Short: "Import a tracked VM target from Terraform outputs or state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(args[0])
			source := strings.TrimSpace(terraformOutputs)
			if source == "" {
				return fmt.Errorf("--terraform-outputs is required")
			}
			resolvedSource, err := app.ResolveTerraformOutputsInput(source)
			if err != nil {
				return err
			}
			outputs, err := app.ReadOutputs(resolvedSource)
			if err != nil {
				return err
			}
			target := app.TrackedTarget{
				Name:                 name,
				InstanceID:           outputString(outputs, "instance_id"),
				PrivateIP:            outputString(outputs, "private_ip"),
				User:                 strings.TrimSpace(user),
				IdentityFile:         strings.TrimSpace(identityFile),
				BastionID:            strings.TrimSpace(bastionID),
				TerraformOutputsPath: resolvedSource,
				LastSeenAt:           time.Now().UTC(),
			}
			if target.BastionID == "" {
				target.BastionID = outputString(outputs, "bastion_id")
			}
			if target.InstanceID == "" {
				return fmt.Errorf("missing output %q in %s", "instance_id", resolvedSource)
			}
			if target.PrivateIP == "" {
				return fmt.Errorf("missing output %q in %s", "private_ip", resolvedSource)
			}
			if err := app.UpsertTrackedTarget(opts.cfg.TrackedTargetsPath, target); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Imported target %s from %s\n", target.Name, resolvedSource)
			return nil
		},
	}
	cmd.Flags().StringVar(&terraformOutputs, "terraform-outputs", "", "Terraform state/outputs file or directory containing terraform.tfstate/outputs.json")
	cmd.Flags().StringVar(&user, "user", "", "Target OS user override")
	cmd.Flags().StringVar(&identityFile, "identity-file", "", "SSH private key for the target VM host alias")
	cmd.Flags().StringVar(&bastionID, "bastion-id", "", "Bastion OCID override for this target")
	return cmd
}

type targetReconcileResult struct {
	Target        targetRow     `json:"target" yaml:"target"`
	Session       *sessionRow   `json:"session,omitempty" yaml:"session,omitempty"`
	SSHConfig     app.SSHConfig `json:"ssh_config" yaml:"ssh_config"`
	SessionSource string        `json:"session_source" yaml:"session_source"`
	Updated       bool          `json:"updated" yaml:"updated"`
}

func newTargetReconcileCmd(opts *rootOptions) *cobra.Command {
	var output string
	var cachedOnly bool
	cmd := &cobra.Command{
		Use:   "reconcile <host>",
		Short: "Create/update a tracked target from active session and effective SSH config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := reconcileTargetFromRuntime(opts.cfg, args[0], !cachedOnly)
			if err != nil {
				return err
			}
			switch strings.ToLower(output) {
			case "", "text", "table":
				fmt.Fprintf(cmd.OutOrStdout(), "Reconciled target %s\n", result.Target.Name)
				fmt.Fprintf(cmd.OutOrStdout(), "Private IP: %s\n", result.Target.PrivateIP)
				fmt.Fprintf(cmd.OutOrStdout(), "Instance: %s\n", result.Target.InstanceID)
				fmt.Fprintf(cmd.OutOrStdout(), "Bastion: %s\n", emptyDash(result.Target.BastionID))
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
	cmd.Flags().BoolVar(&cachedOnly, "cached", false, "Use cached session state only; do not call live OCI APIs")
	cmd.Flags().BoolVar(&cachedOnly, "no-live", false, "Use cached session state only; do not call live OCI APIs")
	return cmd
}

func reconcileTargetFromRuntime(cfg app.Config, host string, live bool) (targetReconcileResult, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return targetReconcileResult{}, fmt.Errorf("host is required")
	}
	sshCfg, err := app.ReadSSHConfig(host)
	if err != nil {
		return targetReconcileResult{}, fmt.Errorf("read effective SSH config for %s: %w", host, err)
	}
	if strings.TrimSpace(sshCfg.HostName) == "" {
		return targetReconcileResult{}, fmt.Errorf("effective SSH config for %s has no HostName", host)
	}
	if strings.TrimSpace(sshCfg.ProxyJump) == "" {
		return targetReconcileResult{}, fmt.Errorf("effective SSH config for %s has no ProxyJump", host)
	}
	session, source, err := usableSessionForReconcile(cfg, live)
	if err != nil {
		return targetReconcileResult{}, err
	}
	if strings.TrimSpace(session.TargetPrivateIP) == "" {
		return targetReconcileResult{}, fmt.Errorf("active session %s has no target private IP; cannot prove SSH HostName maps to the session", session.ID)
	}
	if strings.TrimSpace(session.TargetPrivateIP) != strings.TrimSpace(sshCfg.HostName) {
		return targetReconcileResult{}, fmt.Errorf("effective SSH HostName %s does not match active session target private IP %s", sshCfg.HostName, session.TargetPrivateIP)
	}
	instanceID := strings.TrimSpace(session.TargetResourceID)
	if instanceID == "" {
		return targetReconcileResult{}, fmt.Errorf("active session %s has no target resource ID; pass explicit target track data instead", session.ID)
	}
	bastionID := strings.TrimSpace(session.BastionID)
	if bastionID == "" {
		if current, err := app.LoadCurrent(cfg.CurrentStatePath); err == nil && current != nil {
			bastionID = strings.TrimSpace(current.ID)
		}
	}
	target := app.TrackedTarget{
		Name:         host,
		InstanceID:   instanceID,
		PrivateIP:    strings.TrimSpace(sshCfg.HostName),
		User:         strings.TrimSpace(sshCfg.User),
		IdentityFile: strings.TrimSpace(sshCfg.IdentityFile),
		BastionID:    bastionID,
		LastSeenAt:   time.Now().UTC(),
	}
	if err := app.UpsertTrackedTarget(cfg.TrackedTargetsPath, target); err != nil {
		return targetReconcileResult{}, err
	}
	row := targetRows([]app.TrackedTarget{target})[0]
	return targetReconcileResult{
		Target: row,
		Session: &sessionRow{
			ID:        session.ID,
			Lifecycle: session.LifecycleState,
			Created:   session.TimeCreated.Format(time.RFC3339),
			Expires:   session.TimeExpires.Format(time.RFC3339),
		},
		SSHConfig:     sshCfg,
		SessionSource: source,
		Updated:       true,
	}, nil
}

func usableSessionForReconcile(cfg app.Config, live bool) (app.BastionSession, string, error) {
	cached, err := app.LoadSession(cfg.SessionStatePath)
	if err != nil {
		return app.BastionSession{}, "", err
	}
	if cached == nil {
		return app.BastionSession{}, "", fmt.Errorf("no cached session available")
	}
	if live && strings.TrimSpace(cached.ID) != "" {
		client := app.OCIClient{Profile: cfg.Profile, Region: cfg.Region, AuthMethod: cfg.AuthMethod}
		if s, err := client.GetSession(cached.ID); err == nil && sessionUsableForReconcile(s) {
			_ = app.SaveSession(cfg.SessionStatePath, s)
			return s, "live", nil
		}
	}
	if sessionUsableForReconcile(*cached) {
		return *cached, "cached", nil
	}
	return app.BastionSession{}, "", fmt.Errorf("cached session is not active and unexpired")
}

func sessionUsableForReconcile(s app.BastionSession) bool {
	if !strings.EqualFold(strings.TrimSpace(s.LifecycleState), "ACTIVE") {
		return false
	}
	if !s.TimeExpires.IsZero() && !s.TimeExpires.After(time.Now()) {
		return false
	}
	return strings.TrimSpace(s.ID) != ""
}

func newTargetListCmd(opts *rootOptions) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tracked VM targets",
		RunE: func(cmd *cobra.Command, _ []string) error {
			targets, err := app.LoadTrackedTargets(opts.cfg.TrackedTargetsPath)
			if err != nil {
				return err
			}
			rows := targetRows(targets)
			switch strings.ToLower(output) {
			case "", "table":
				if len(rows) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No tracked targets found")
					return nil
				}
				for _, r := range rows {
					fmt.Fprintf(cmd.OutOrStdout(), "%s  instance=%s  private_ip=%s  user=%s  identity=%s  bastion=%s\n",
						r.Name, r.InstanceID, r.PrivateIP, dash(r.User), dash(r.IdentityFile), dash(r.BastionID))
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
	return cmd
}

func newTargetShowCmd(opts *rootOptions) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show one tracked VM target",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target, err := app.FindTrackedTarget(opts.cfg.TrackedTargetsPath, args[0])
			if err != nil {
				return err
			}
			if target == nil {
				return fmt.Errorf("tracked target %q not found", strings.TrimSpace(args[0]))
			}
			row := targetRows([]app.TrackedTarget{*target})[0]
			switch strings.ToLower(output) {
			case "", "table":
				fmt.Fprintf(cmd.OutOrStdout(), "%s  instance=%s  private_ip=%s  user=%s  identity=%s  bastion=%s\n",
					row.Name, row.InstanceID, row.PrivateIP, dash(row.User), dash(row.IdentityFile), dash(row.BastionID))
				return nil
			case "json":
				return printJSON(row)
			case "yaml", "yml":
				return printYAML(row)
			default:
				return fmt.Errorf("unsupported output format: %s", output)
			}
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table|json|yaml")
	return cmd
}

func newTargetRmCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <name> [more...]",
		Short: "Remove one or more tracked VM targets",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			names := make([]string, 0, len(args))
			for _, arg := range args {
				name := strings.TrimSpace(arg)
				if name != "" {
					names = append(names, name)
				}
			}
			targets, err := app.LoadTrackedTargets(opts.cfg.TrackedTargetsPath)
			if err != nil {
				return err
			}
			known := map[string]bool{}
			for _, target := range targets {
				known[target.Name] = true
			}
			for _, name := range names {
				if !known[name] {
					return fmt.Errorf("tracked target %q not found", name)
				}
			}
			removed, err := app.RemoveTrackedTarget(opts.cfg.TrackedTargetsPath, names...)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %d tracked target(s)\n", removed)
			return nil
		},
	}
	return cmd
}

func targetRows(targets []app.TrackedTarget) []targetRow {
	rows := make([]targetRow, 0, len(targets))
	for _, target := range targets {
		rows = append(rows, targetRow{
			Name:                 target.Name,
			InstanceID:           target.InstanceID,
			PrivateIP:            target.PrivateIP,
			User:                 target.User,
			IdentityFile:         target.IdentityFile,
			BastionID:            target.BastionID,
			TerraformOutputsPath: target.TerraformOutputsPath,
			LastSeenAt:           target.LastSeenAt.Format(time.RFC3339),
		})
	}
	return rows
}

func dash(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}

func outputString(outputs map[string]any, key string) string {
	if v, ok := outputs[key]; ok {
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
	return ""
}
