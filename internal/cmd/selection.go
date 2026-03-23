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
