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
	if (cfg.Profile == "" || cfg.Profile == app.DefaultProfile) && strings.TrimSpace(cur.Profile) != "" {
		cfg.Profile = cur.Profile
	}
	if (cfg.Region == "" || cfg.Region == app.DefaultRegion) && strings.TrimSpace(cur.Region) != "" {
		cfg.Region = cur.Region
	}
	return cur, nil
}

func requireBastionID(current *app.CurrentBastion, explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit), nil
	}
	if current != nil && strings.TrimSpace(current.ID) != "" {
		return strings.TrimSpace(current.ID), nil
	}
	return "", fmt.Errorf("no bastion selected; use `bastion-session use <id>` or pass --bastion-id")
}
