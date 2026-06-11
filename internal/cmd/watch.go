package cmd

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/adrianmross/bastion-session/internal/app"
	"github.com/spf13/cobra"
)

const defaultWatchMaxTargets = 20
const defaultWatchSessionTTL = 24 * time.Hour

func defaultWatchSessionTTLText() string {
	return defaultWatchSessionTTL.String()
}

type watchTarget struct {
	Label      string
	BastionID  string
	HostAlias  string
	InstanceID string
	PrivateIP  string
	Config     app.Config
}

type watchRefreshResult struct {
	Target  watchTarget
	Session app.BastionSession
}

type watchRefresher func(app.Config, app.RefreshOptions) (app.BastionSession, error)

func newWatchCmd(opts *rootOptions) *cobra.Command {
	var interval int
	var source string
	var maxTargets int
	var sessionTTLText string
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Continuously refresh bastion session",
		RunE: func(cmd *cobra.Command, _ []string) error {
			sessionTTL, err := parseSessionTTL(sessionTTLText)
			if err != nil {
				return err
			}
			var explicit time.Duration
			if interval > 0 {
				explicit = time.Duration(interval) * time.Second
			}
			sleepFor := app.DefaultWatchInterval
			if explicit > 0 {
				sleepFor = explicit
			}
			for {
				results, err := runWatchIteration(opts.cfg, source, maxTargets, sessionTTL, app.RefreshSessionWithTarget, os.Stdout, os.Stderr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Watch refresh failed: %v\n", err)
					sleepFor = app.DefaultWatchInterval
					if explicit > 0 {
						sleepFor = explicit
					}
				} else if explicit == 0 {
					sleepFor = watchAutoRefreshInterval(results)
				}
				fmt.Fprintf(os.Stdout, "Sleeping for %d seconds\n", int(sleepFor.Seconds()))
				time.Sleep(sleepFor)
			}
		},
	}
	cmd.Flags().IntVarP(&interval, "interval", "i", 0, "Refresh interval in seconds")
	cmd.Flags().StringVar(&source, "source", "current", "Watch source: current, tracked, or all")
	cmd.Flags().IntVar(&maxTargets, "max-targets", defaultWatchMaxTargets, "Maximum tracked targets to refresh per interval")
	cmd.Flags().StringVar(&sessionTTLText, "session-ttl", defaultWatchSessionTTLText(), "Requested TTL for newly created sessions as a duration or seconds")
	return cmd
}

func runWatchIteration(base app.Config, source string, maxTargets int, sessionTTL time.Duration, refresh watchRefresher, stdout, stderr io.Writer) ([]watchRefreshResult, error) {
	targets, err := loadWatchTargets(base, source, maxTargets)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no watch targets found for source %q", normalizedWatchSource(source))
	}

	results := make([]watchRefreshResult, 0, len(targets))
	failures := 0
	for _, target := range targets {
		fmt.Fprintf(stdout, "Refreshing %s (bastion=%s profile=%s region=%s auth=%s host=%s)\n",
			target.Label, target.BastionID, target.Config.Profile, target.Config.Region, authLabel(target.Config.AuthMethod), target.HostAlias)
		session, err := refresh(target.Config, app.RefreshOptions{
			BastionID:  target.BastionID,
			InstanceID: target.InstanceID,
			PrivateIP:  target.PrivateIP,
			SessionTTL: sessionTTL,
		})
		if err != nil {
			failures++
			fmt.Fprintln(stderr, formatWatchFailure(target, err))
			continue
		}
		results = append(results, watchRefreshResult{Target: target, Session: session})
		fmt.Fprintf(stdout, "Refreshed %s: session=%s lifecycle=%s expires=%s\n",
			target.Label, session.ID, session.LifecycleState, session.TimeExpires.Format(time.RFC3339))
	}
	if len(results) > 0 && (len(results) > 1 || normalizedWatchSource(source) != "current") {
		if err := ensureWatchSSHFragment(base.SSHIncludePath, results); err != nil {
			return results, err
		}
	}
	if len(results) == 0 && failures > 0 {
		return results, fmt.Errorf("all %d watch target refreshes failed", failures)
	}
	return results, nil
}

func loadWatchTargets(base app.Config, source string, maxTargets int) ([]watchTarget, error) {
	source = normalizedWatchSource(source)
	if source != "current" && source != "tracked" && source != "all" {
		return nil, fmt.Errorf("invalid watch source %q; expected current, tracked, or all", source)
	}
	if maxTargets <= 0 {
		maxTargets = defaultWatchMaxTargets
	}
	targets := []watchTarget{}
	seen := map[string]bool{}

	if source == "current" || source == "all" {
		cfg := base
		cur, err := loadCurrentSelection(&cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to load current selection: %w", err)
		}
		applyCurrentSelectionIdentity(&cfg, cur)
		if cur != nil && strings.TrimSpace(cur.ID) != "" {
			id := strings.TrimSpace(cur.ID)
			seen[id] = true
			targets = append(targets, watchTarget{
				Label:     "current",
				BastionID: id,
				HostAlias: fmt.Sprintf("%s-bastion", cfg.Profile),
				Config:    cfg,
			})
		} else {
			targets = append(targets, watchTarget{
				Label:     "current",
				HostAlias: fmt.Sprintf("%s-bastion", cfg.Profile),
				Config:    cfg,
			})
		}
	}

	if source == "tracked" || source == "all" {
		tracked, err := app.LoadTrackedTargets(base.TrackedTargetsPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load tracked targets: %w", err)
		}
		names := make([]string, 0, len(tracked))
		for _, t := range tracked {
			if strings.TrimSpace(t.Name) != "" {
				names = append(names, strings.TrimSpace(t.Name))
			}
		}
		refs := app.BuildShortRefs(names, 2)
		added := 0
		for _, t := range tracked {
			name := strings.TrimSpace(t.Name)
			bastionID := strings.TrimSpace(t.BastionID)
			instanceID := strings.TrimSpace(t.InstanceID)
			privateIP := strings.TrimSpace(t.PrivateIP)
			key := name + "|" + bastionID + "|" + instanceID + "|" + privateIP
			if bastionID == "" || instanceID == "" || privateIP == "" || seen[key] {
				continue
			}
			if added >= maxTargets {
				break
			}
			cfg := configForTrackedWatchTarget(base, t)
			ref := refs[name]
			alias := trackedWatchHostAlias(t, ref)
			targets = append(targets, watchTarget{
				Label:      trackedWatchLabel(t, ref),
				BastionID:  bastionID,
				HostAlias:  alias,
				InstanceID: instanceID,
				PrivateIP:  privateIP,
				Config:     cfg,
			})
			seen[key] = true
			added++
		}
	}

	return uniquifyWatchHostAliases(targets), nil
}

func normalizedWatchSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "", "current":
		return "current"
	case "tracked":
		return "tracked"
	case "all":
		return "all"
	default:
		return source
	}
}

func configForTrackedWatchTarget(base app.Config, t app.TrackedTarget) app.Config {
	cfg := base
	if v := strings.TrimSpace(t.Profile); v != "" {
		cfg.Profile = v
	}
	if v := strings.TrimSpace(t.Region); v != "" {
		cfg.Region = v
	}
	if v := strings.TrimSpace(t.AuthMethod); v != "" {
		cfg.AuthMethod = v
	}
	if v := strings.TrimSpace(t.SSHPublicKey); v != "" {
		cfg.SSHPublicKey = v
	}
	if v := strings.TrimSpace(t.User); v != "" {
		cfg.TargetUser = v
	}
	if v := strings.TrimSpace(t.IdentityFile); v != "" {
		cfg.SSHPrivateKey = v
	}
	cfg.ScopedContext = nil
	cfg.ContextScopeEnabled = false
	return cfg
}

var watchAliasUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func trackedWatchHostAlias(t app.TrackedTarget, ref string) string {
	if name := strings.Trim(watchAliasUnsafe.ReplaceAllString(strings.ToLower(strings.TrimSpace(t.Name)), "-"), "-"); name != "" {
		if strings.HasSuffix(name, "-bastion") || name == "bastion" {
			return name
		}
		return name + "-bastion"
	}
	profile := strings.Trim(watchAliasUnsafe.ReplaceAllString(strings.ToLower(strings.TrimSpace(t.Profile)), "-"), "-")
	if profile == "" {
		profile = "tracked"
	}
	if ref == "" {
		ref = "bastion"
	}
	return profile + "-" + ref + "-bastion"
}

func trackedWatchLabel(t app.TrackedTarget, ref string) string {
	if name := strings.TrimSpace(t.Name); name != "" {
		return name
	}
	if ref != "" {
		return ref
	}
	if ip := strings.TrimSpace(t.PrivateIP); ip != "" {
		return ip
	}
	return strings.TrimSpace(t.InstanceID)
}

func uniquifyWatchHostAliases(targets []watchTarget) []watchTarget {
	seen := map[string]int{}
	for i := range targets {
		alias := strings.TrimSpace(targets[i].HostAlias)
		if alias == "" {
			alias = "bastion"
		}
		seen[alias]++
		if seen[alias] > 1 {
			alias = fmt.Sprintf("%s-%d", alias, seen[alias])
		}
		targets[i].HostAlias = alias
	}
	return targets
}

func ensureWatchSSHFragment(path string, results []watchRefreshResult) error {
	hosts := make([]app.SSHHostEntry, 0, len(results))
	for _, result := range results {
		hosts = append(hosts, app.SSHHostEntry{
			Alias:         result.Target.HostAlias,
			Profile:       result.Target.Config.Profile,
			Region:        result.Target.Config.Region,
			SessionID:     result.Session.ID,
			SSHPublicKey:  result.Target.Config.SSHPublicKey,
			SSHPrivateKey: result.Target.Config.SSHPrivateKey,
		})
	}
	return app.UpdateSSHFragmentForHosts(path, hosts)
}

func formatWatchFailure(target watchTarget, err error) string {
	return fmt.Sprintf("Failed to refresh %s (bastion=%s profile=%s region=%s auth=%s host=%s): %v",
		target.Label, target.BastionID, target.Config.Profile, target.Config.Region, authLabel(target.Config.AuthMethod), target.HostAlias, err)
}

func authLabel(auth string) string {
	if strings.TrimSpace(auth) == "" {
		return "default"
	}
	return strings.TrimSpace(auth)
}

func watchAutoRefreshInterval(results []watchRefreshResult) time.Duration {
	if len(results) == 0 {
		return app.DefaultWatchInterval
	}
	next := app.AutoRefreshInterval(results[0].Session)
	for _, result := range results[1:] {
		if interval := app.AutoRefreshInterval(result.Session); interval < next {
			next = interval
		}
	}
	return next
}
