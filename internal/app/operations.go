package app

import (
	"fmt"
	"strings"
	"time"
)

type Status struct {
	SessionID string `json:"session_id" yaml:"session_id"`
	Lifecycle string `json:"lifecycle" yaml:"lifecycle"`
	Expires   string `json:"expires" yaml:"expires"`
	ExpiresIn string `json:"expires_in" yaml:"expires_in"`
	Profile   string `json:"profile" yaml:"profile"`
	Region    string `json:"region" yaml:"region"`
	Context   string `json:"context,omitempty" yaml:"context,omitempty"`
}

type RefreshOptions struct {
	BastionID  string
	InstanceID string
	PrivateIP  string
}

func RefreshSessionWithTarget(cfg Config, opts RefreshOptions) (BastionSession, error) {
	metadata := SessionMetadata{}
	if strings.TrimSpace(opts.BastionID) == "" || strings.TrimSpace(opts.InstanceID) == "" || strings.TrimSpace(opts.PrivateIP) == "" {
		fromTF, err := LoadTargetDetails(cfg)
		if err != nil {
			return BastionSession{}, err
		}
		metadata = fromTF
	}
	if strings.TrimSpace(opts.BastionID) != "" {
		metadata.BastionID = strings.TrimSpace(opts.BastionID)
	}
	if strings.TrimSpace(opts.InstanceID) != "" {
		metadata.InstanceID = strings.TrimSpace(opts.InstanceID)
	}
	if strings.TrimSpace(opts.PrivateIP) != "" {
		metadata.PrivateIP = strings.TrimSpace(opts.PrivateIP)
	}
	if metadata.BastionID == "" || metadata.InstanceID == "" || metadata.PrivateIP == "" {
		return BastionSession{}, fmt.Errorf("bastion_id, instance_id, and private_ip are required")
	}
	pub := cfg.SSHPublicKey
	if strings.TrimSpace(pub) == "" {
		pub = ResolvePublicKey(cfg)
	}
	if strings.TrimSpace(pub) == "" {
		return BastionSession{}, fmt.Errorf("SSH_PUBLIC_KEY environment variable or config required")
	}
	cfg.SSHPublicKey = pub

	client := OCIClient{Profile: cfg.Profile, Region: cfg.Region, AuthMethod: cfg.AuthMethod}
	created, err := client.CreateSession(TargetDetails{
		BastionID:     metadata.BastionID,
		InstanceID:    metadata.InstanceID,
		PrivateIP:     metadata.PrivateIP,
		TargetUser:    cfg.TargetUser,
		PublicKeyPath: pub,
	})
	if err != nil {
		return BastionSession{}, err
	}
	active, err := WaitForActive(client, created.ID, ActiveWaitTimeout, ActivePollIntervalSeconds)
	if err != nil {
		return BastionSession{}, err
	}
	if err := SaveSession(cfg.SessionStatePath, active); err != nil {
		return BastionSession{}, err
	}
	if err := EnsureSSHInclude(cfg.SSHIncludePath); err != nil {
		return BastionSession{}, err
	}
	if err := UpdateSSHFragment(cfg, active.ID); err != nil {
		return BastionSession{}, err
	}
	_ = UpsertTracked(cfg.TrackedBastionsPath, TrackedBastion{
		ID:      metadata.BastionID,
		Region:  cfg.Region,
		Profile: cfg.Profile,
		CompartmentID: func() string {
			if cfg.ScopedContext != nil {
				return cfg.ScopedContext.CompartmentOCID
			}
			return ""
		}(),
		ContextName: func() string {
			if cfg.ScopedContext != nil {
				return cfg.ScopedContext.Name
			}
			return ""
		}(),
		LastSeenAt: time.Now().UTC(),
	})
	return active, nil
}

func RefreshSession(cfg Config) (BastionSession, error) {
	return RefreshSessionWithTarget(cfg, RefreshOptions{})
}

func SessionStatus(cfg Config) (Status, error) {
	cached, err := LoadSession(cfg.SessionStatePath)
	if err != nil {
		return Status{}, err
	}
	if cached == nil {
		return Status{}, fmt.Errorf("no cached session. Run refresh first")
	}
	client := OCIClient{Profile: cfg.Profile, Region: cfg.Region, AuthMethod: cfg.AuthMethod}
	s := *cached
	if live, err := client.GetSession(cached.ID); err == nil {
		s = live
		_ = SaveSession(cfg.SessionStatePath, live)
	}
	st := Status{
		SessionID: s.ID,
		Lifecycle: s.LifecycleState,
		Expires:   s.TimeExpires.Format(time.RFC3339),
		ExpiresIn: s.ExpiresIn().String(),
		Profile:   cfg.Profile,
		Region:    cfg.Region,
	}
	if cfg.ScopedContext != nil {
		st.Context = cfg.ScopedContext.Name
	}
	return st, nil
}

func ListScopedBastions(cfg Config) ([]BastionInfo, error) {
	client := OCIClient{Profile: cfg.Profile, Region: cfg.Region, AuthMethod: cfg.AuthMethod}
	compartment := ""
	if cfg.ScopedContext != nil {
		compartment = cfg.ScopedContext.CompartmentOCID
	}
	items, err := client.ListBastions(compartment)
	if err != nil {
		return nil, err
	}
	tracked := make([]TrackedBastion, 0, len(items))
	ctxName := ""
	if cfg.ScopedContext != nil {
		ctxName = cfg.ScopedContext.Name
	}
	for i := range items {
		items[i].ScopeContext = ctxName
		tracked = append(tracked, TrackedBastion{
			ID:            items[i].ID,
			Name:          items[i].Name,
			CompartmentID: items[i].CompartmentID,
			Region:        cfg.Region,
			Profile:       cfg.Profile,
			ContextName:   ctxName,
			LastSeenAt:    time.Now().UTC(),
		})
	}
	if len(tracked) > 0 {
		_ = UpsertTracked(cfg.TrackedBastionsPath, tracked...)
	}
	return items, nil
}
