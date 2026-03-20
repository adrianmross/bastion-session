package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var version = "dev"

type rootOptions struct {
	profile          string
	region           string
	authMethod       string
	targetUser       string
	sshPublicKey     string
	sshPrivateKey    string
	sshInclude       string
	statePath        string
	terraformOutputs string
	trackedPath      string

	ociContextConfig string
	globalOCIContext bool
	noContextScope   bool

	cfg app.Config
}

func newRootCmd() *cobra.Command {
	opts := &rootOptions{}
	cmd := &cobra.Command{
		Use:           "bastion-session",
		Short:         "OCI bastion session manager (Go rewrite with context-aware CLI/TUI)",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cfg := app.ConfigFromEnv()
			if opts.profile != "" {
				cfg.Profile = opts.profile
			}
			if opts.region != "" {
				cfg.Region = opts.region
			}
			if opts.authMethod != "" {
				cfg.AuthMethod = opts.authMethod
			}
			if opts.targetUser != "" {
				cfg.TargetUser = opts.targetUser
			}
			if opts.sshPublicKey != "" {
				cfg.SSHPublicKey = opts.sshPublicKey
			}
			if opts.sshPrivateKey != "" {
				cfg.SSHPrivateKey = opts.sshPrivateKey
			}
			if opts.sshInclude != "" {
				cfg.SSHIncludePath = opts.sshInclude
			}
			if opts.statePath != "" {
				cfg.SessionStatePath = opts.statePath
			}
			if opts.terraformOutputs != "" {
				cfg.TerraformOutputsPath = opts.terraformOutputs
			}
			if opts.trackedPath != "" {
				cfg.TrackedBastionsPath = opts.trackedPath
			}
			cfg.OCIContextConfigPath = opts.ociContextConfig
			cfg.UseGlobalOCIContext = opts.globalOCIContext
			cfg.ContextScopeEnabled = !opts.noContextScope

			if cfg.ContextScopeEnabled {
				ctxPath, err := app.ResolveOCIContextConfigPath(cfg.OCIContextConfigPath, cfg.UseGlobalOCIContext)
				if err == nil {
					cfg.OCIContextConfigPath = ctxPath
					if ctxRef, err := app.LoadCurrentOCIContext(ctxPath); err == nil {
						cfg.ApplyContextScope(ctxRef)
					}
				}
			}
			opts.cfg = cfg
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = args
			if v, _ := cmd.Flags().GetBool("version"); v {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), version)
				return err
			}
			return cmd.Help()
		},
	}
	cmd.Version = version
	cmd.Flags().BoolP("version", "v", false, "Print version")

	pf := cmd.PersistentFlags()
	pf.StringVarP(&opts.profile, "profile", "p", "", "OCI profile name")
	pf.StringVarP(&opts.region, "region", "r", "", "OCI region identifier")
	pf.StringVarP(&opts.authMethod, "auth-method", "a", "", "OCI auth method")
	pf.StringVarP(&opts.targetUser, "target-user", "u", "", "Target OS user")
	pf.StringVarP(&opts.sshPublicKey, "ssh-public-key", "P", "", "Path to SSH public key")
	pf.StringVarP(&opts.sshPrivateKey, "ssh-private-key", "K", "", "Path to SSH private key")
	pf.StringVarP(&opts.sshInclude, "ssh-include", "I", "", "Path to SSH include fragment")
	pf.StringVar(&opts.statePath, "state-path", "", "Path to state cache file")
	pf.StringVar(&opts.trackedPath, "tracked-path", "", "Path to tracked bastions file")
	pf.StringVar(&opts.terraformOutputs, "terraform-outputs", "", "Path to Terraform state/outputs file")
	pf.StringVar(&opts.ociContextConfig, "oci-context-config", "", "Path to oci-context config file")
	pf.BoolVarP(&opts.globalOCIContext, "global", "g", false, "Use global oci-context config (~/.oci-context/config.yml)")
	pf.BoolVar(&opts.noContextScope, "no-context-scope", false, "Disable oci-context-based scoping")

	cmd.AddCommand(
		newRefreshCmd(opts),
		newStatusCmd(opts),
		newWatchCmd(opts),
		newListBastionsCmd(opts),
		newTUICmd(opts),
	)
	return cmd
}

func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printYAML(v any) error {
	enc := yaml.NewEncoder(os.Stdout)
	defer enc.Close()
	return enc.Encode(v)
}
