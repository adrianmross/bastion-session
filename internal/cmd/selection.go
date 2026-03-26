package cmd

import (
	"fmt"
	"strings"

	"github.com/adrianmross/bastion-session/internal/app"
)

func loadCurrentSelection(cfg *app.Config) (*app.CurrentBastion, error) {
	cur, err := app.LoadCurrent(cfg.CurrentStatePath)
	if err != nil {
		return nil, err
	}
	if cur == nil {
		return nil, nil
	}
	// When oci-context scoping is active, keep scoped profile/region/auth authoritative
	// and avoid importing potentially stale tracked metadata into command execution config.
	allowIdentityOverrideFromCurrent := cfg.ScopedContext == nil
	if allowIdentityOverrideFromCurrent && (cfg.Profile == "" || cfg.Profile == app.DefaultProfile) && strings.TrimSpace(cur.Profile) != "" {
		cfg.Profile = cur.Profile
	}
	if allowIdentityOverrideFromCurrent && (cfg.Region == "" || cfg.Region == app.DefaultRegion) && strings.TrimSpace(cur.Region) != "" {
		cfg.Region = cur.Region
	}
	if allowIdentityOverrideFromCurrent && strings.TrimSpace(cfg.AuthMethod) == "" && strings.TrimSpace(cur.AuthMethod) != "" {
		cfg.AuthMethod = cur.AuthMethod
	}
	if strings.TrimSpace(cfg.SSHPublicKey) == "" && strings.TrimSpace(cur.SSHPublicKey) != "" {
		cfg.SSHPublicKey = cur.SSHPublicKey
	}
	return cur, nil
}

func applyCurrentSelectionIdentity(cfg *app.Config, cur *app.CurrentBastion) {
	if cfg == nil || cur == nil {
		return
	}
	if v := strings.TrimSpace(cur.Profile); v != "" {
		cfg.Profile = v
	}
	if v := strings.TrimSpace(cur.Region); v != "" {
		cfg.Region = v
	}
	if v := strings.TrimSpace(cur.AuthMethod); v != "" {
		cfg.AuthMethod = v
	}
	if v := strings.TrimSpace(cur.SSHPublicKey); v != "" {
		cfg.SSHPublicKey = v
	}
}

func requireBastionID(current *app.CurrentBastion, explicit string) (string, error) {
	if token := strings.TrimSpace(explicit); token != "" {
		if strings.HasPrefix(token, "ocid1.") {
			return token, nil
		}
		if current != nil && strings.TrimSpace(current.ID) != "" {
			ref := app.BuildShortRefs([]string{current.ID}, 2)[current.ID]
			if token == ref {
				return strings.TrimSpace(current.ID), nil
			}
		}
		return token, nil
	}
	if current != nil && strings.TrimSpace(current.ID) != "" {
		return strings.TrimSpace(current.ID), nil
	}
	return "", fmt.Errorf("no bastion selected; use `bastion-session use <id>` or pass --bastion-id")
}

func resolveBastionIDToken(cfg *app.Config, token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", fmt.Errorf("empty bastion token")
	}
	if strings.HasPrefix(token, "ocid1.") {
		return token, nil
	}

	cur, err := app.LoadCurrent(cfg.CurrentStatePath)
	if err != nil {
		return "", err
	}
	if cur != nil && strings.TrimSpace(cur.ID) != "" {
		ref := app.BuildShortRefs([]string{cur.ID}, 2)[cur.ID]
		if cur.ID == token || ref == token {
			return cur.ID, nil
		}
	}

	tracked, err := app.LoadTracked(cfg.TrackedBastionsPath)
	if err != nil {
		return "", err
	}
	if len(tracked) > 0 {
		ids := make([]string, 0, len(tracked))
		for _, b := range tracked {
			ids = append(ids, b.ID)
		}
		refs := app.BuildShortRefs(ids, 2)
		for _, b := range tracked {
			if b.ID == token || refs[b.ID] == token {
				return b.ID, nil
			}
		}
	}

	if cfg.ContextScopeEnabled {
		scoped, err := app.ListScopedBastions(*cfg)
		if err == nil && len(scoped) > 0 {
			ids := make([]string, 0, len(scoped))
			for _, b := range scoped {
				ids = append(ids, b.ID)
			}
			refs := app.BuildShortRefs(ids, 2)
			for _, b := range scoped {
				if b.ID == token || refs[b.ID] == token {
					return b.ID, nil
				}
			}
		}
	}

	return "", fmt.Errorf("bastion %q not found in current/tracked%s", token, func() string {
		if cfg.ContextScopeEnabled {
			return "/scoped"
		}
		return ""
	}())
}
