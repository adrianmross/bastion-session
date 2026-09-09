package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

type forwardResult struct {
	Ready            bool   `json:"ready" yaml:"ready"`
	SSHCommand       string `json:"ssh_command" yaml:"ssh_command"`
	SessionID        string `json:"session_id" yaml:"session_id"`
	SessionLifecycle string `json:"session_lifecycle" yaml:"session_lifecycle"`
	ExpiresAt        string `json:"expires_at" yaml:"expires_at"`
	LocalPort        int    `json:"local_port" yaml:"local_port"`
	TargetPrivateIP  string `json:"target_private_ip" yaml:"target_private_ip"`
	TargetPort       int    `json:"target_port" yaml:"target_port"`
	Profile          string `json:"profile" yaml:"profile"`
	Region           string `json:"region" yaml:"region"`
}

// newForwardCmd exposes OCI's port-forwarding session type.
//
// `connect` and `ensure` create MANAGED_SSH sessions, which reach a compute instance as an
// OS user and require the Bastion plugin enabled on that instance. Plenty of things worth
// reaching are neither: a private OKE API endpoint, a database, an internal HTTP service.
// Those need a port-forwarding session, which has no plugin requirement and no target
// user -- just an address and a port inside the bastion's VCN.
func newForwardCmd(opts *rootOptions) *cobra.Command {
	var bastionID string
	var privateIP string
	var targetPort int
	var localPort int
	var keyOverride string
	var output string
	var sessionTTLText string
	var displayName string
	var dryRun bool
	var waitTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "forward [bastion-ref-or-ocid]",
		Short: "Open a port-forwarding session to a private address in the bastion's VCN",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if keyOverride != "" {
				opts.cfg.SSHPublicKey = keyOverride
			}
			if strings.TrimSpace(privateIP) == "" {
				return fmt.Errorf("--private-ip is required")
			}
			if targetPort <= 0 {
				return fmt.Errorf("--target-port is required")
			}
			if localPort <= 0 {
				localPort = targetPort
			}
			sessionTTL, err := parseSessionTTL(sessionTTLText)
			if err != nil {
				return err
			}
			structuredOutput := strings.EqualFold(output, "json") ||
				strings.EqualFold(output, "yaml") || strings.EqualFold(output, "yml")

			if len(args) == 1 {
				if strings.TrimSpace(bastionID) != "" {
					return fmt.Errorf("positional bastion token cannot be used with --bastion-id")
				}
				resolved, err := resolveBastionIDToken(&opts.cfg, args[0])
				if err != nil {
					return err
				}
				bastionID = resolved
			}
			cur, err := loadCurrentSelection(&opts.cfg)
			if err != nil {
				return err
			}
			applyCurrentSelectionIdentity(&opts.cfg, cur)
			bid, err := requireBastionID(cur, bastionID)
			if err != nil {
				return err
			}

			publicKey := app.ResolvePublicKey(opts.cfg)
			if strings.TrimSpace(publicKey) == "" {
				return fmt.Errorf("could not resolve an SSH public key; pass --ssh-public-key")
			}

			client := app.OCIClientFromConfig(opts.cfg)
			if displayName == "" {
				displayName = fmt.Sprintf("forward-%s-%d", strings.ReplaceAll(privateIP, ".", "-"), targetPort)
			}
			created, err := client.CreateForwardSession(app.ForwardTarget{
				BastionID:     bid,
				PrivateIP:     privateIP,
				Port:          targetPort,
				PublicKeyPath: publicKey,
				SessionTTL:    sessionTTL,
				DisplayName:   displayName,
			})
			if err != nil {
				return err
			}
			if waitTimeout <= 0 {
				waitTimeout = app.ActiveWaitTimeout
			}
			session, err := app.WaitForActive(client, created.ID, waitTimeout, app.ActivePollIntervalSeconds, nil)
			if err != nil {
				return err
			}

			sshHost := fmt.Sprintf("%s@host.bastion.%s.oci.oraclecloud.com", session.ID, opts.cfg.Region)
			sshArgs := app.ForwardSSHArgs(opts.cfg, session.ID, localPort, privateIP, targetPort)
			result := forwardResult{
				SessionID:        session.ID,
				SessionLifecycle: session.LifecycleState,
				ExpiresAt:        session.TimeExpires.Format(time.RFC3339),
				LocalPort:        localPort,
				TargetPrivateIP:  privateIP,
				TargetPort:       targetPort,
				Profile:          opts.cfg.Profile,
				Region:           opts.cfg.Region,
				SSHCommand:       "ssh " + strings.Join(sshArgs, " "),
			}

			if dryRun {
				result.Ready = strings.EqualFold(session.LifecycleState, "ACTIVE")
				return emitForward(cmd, output, result, true)
			}

			// ACTIVE is not enough -- the SSH front door rejects the key for several
			// seconds afterwards. Probing here turns a misleading
			// "Permission denied (publickey)" into a wait.
			probe := app.SSHForwardProbe(opts.cfg, privateIP, targetPort, probePort(localPort))
			if err := app.WaitForSSHReady(session.ID, probe, app.SSHReadyTimeout, app.SSHReadyPollInterval,
				func(attempt int) {
					if attempt > 1 && !structuredOutput {
						fmt.Fprintf(cmd.ErrOrStderr(), "waiting for the session's SSH endpoint (attempt %d)\n", attempt)
					}
				}); err != nil {
				return err
			}
			result.Ready = true

			if structuredOutput {
				return emitForward(cmd, output, result, true)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "forwarding 127.0.0.1:%d -> %s:%d via %s\n", localPort, privateIP, targetPort, sshHost)
			fmt.Fprintf(cmd.OutOrStdout(), "session %s (expires %s). Ctrl-C to close.\n", session.ID, result.ExpiresAt)

			ssh := exec.Command("ssh", sshArgs...)
			ssh.Stdin = os.Stdin
			ssh.Stdout = cmd.OutOrStdout()
			ssh.Stderr = cmd.ErrOrStderr()
			return ssh.Run()
		},
	}

	cmd.Flags().StringVar(&bastionID, "bastion-id", "", "Bastion OCID (defaults to current selected bastion)")
	cmd.Flags().StringVar(&privateIP, "private-ip", "", "Target private IP inside the bastion's VCN (required)")
	cmd.Flags().IntVar(&targetPort, "target-port", 0, "Target port on the private IP (required)")
	cmd.Flags().IntVar(&localPort, "local-port", 0, "Local port to bind (defaults to --target-port)")
	cmd.Flags().StringVar(&keyOverride, "key", "", "SSH public key path override when creating a new session")
	cmd.Flags().StringVarP(&output, "output", "o", "text", "Output format: text|json|yaml")
	cmd.Flags().StringVar(&sessionTTLText, "session-ttl", "", "Requested TTL for the session as a duration or seconds (e.g. 3h, 10800)")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Session display name (defaults to forward-<ip>-<port>)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Create the session and print the ssh command without connecting")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", app.ActiveWaitTimeout, "How long to wait for a new session to reach ACTIVE (e.g. 2m, 10m)")
	return cmd
}

// probePort keeps the readiness probe off the port the real tunnel wants, so a probe still
// holding its socket cannot make the tunnel fail with "Address already in use".
func probePort(localPort int) int {
	p := localPort + 1 + rand.Intn(200)
	if p > 65535 {
		p = localPort - 1
	}
	return p
}

// emitForward renders in whichever format was asked for, matching connect's contract.
func emitForward(cmd *cobra.Command, output string, result forwardResult, structured bool) error {
	switch strings.ToLower(output) {
	case "", "text", "table":
		if structured {
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", result.SSHCommand)
			return nil
		}
		return nil
	case "json":
		return printJSON(result)
	case "yaml", "yml":
		return printYAML(result)
	default:
		return fmt.Errorf("unsupported output format: %s", output)
	}
}
