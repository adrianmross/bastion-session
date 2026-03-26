package app

import (
	"fmt"
	"slices"
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
	BastionID   string
	InstanceID  string
	PrivateIP   string
	WaitTimeout time.Duration
	OnCreated   func(BastionSession)
	OnReused    func(BastionSession)
	OnPoll      func(BastionSession)
}

const MinReusableSessionTTL = 2 * time.Minute

func RefreshSessionWithTarget(cfg Config, opts RefreshOptions) (BastionSession, error) {
	client := OCIClient{Profile: cfg.Profile, Region: cfg.Region, AuthMethod: cfg.AuthMethod}
	metadata, err := resolveTargetMetadata(cfg, client, opts)
	if err != nil {
		return BastionSession{}, err
	}
	pub := cfg.SSHPublicKey
	if strings.TrimSpace(pub) == "" {
		pub = ResolvePublicKey(cfg)
	}
	if strings.TrimSpace(pub) == "" {
		return BastionSession{}, fmt.Errorf("SSH_PUBLIC_KEY environment variable or config required")
	}
	cfg.SSHPublicKey = pub

	now := time.Now().UTC()
	if reusableID, ok := selectReusableSession(metadata, listActiveSessions(client, metadata.BastionID), now, MinReusableSessionTTL); ok {
		if reused, err := client.GetSession(reusableID); err == nil {
			if sessionIsReusable(metadata, reused, now, MinReusableSessionTTL) {
				if opts.OnReused != nil {
					opts.OnReused(reused)
				}
				if err := SaveSession(cfg.SessionStatePath, reused); err != nil {
					return BastionSession{}, err
				}
				if err := EnsureSSHInclude(cfg.SSHIncludePath); err != nil {
					return BastionSession{}, err
				}
				if err := UpdateSSHFragment(cfg, reused.ID); err != nil {
					return BastionSession{}, err
				}
				_ = UpsertTracked(cfg.TrackedBastionsPath, TrackedBastion{
					ID:         metadata.BastionID,
					Region:     cfg.Region,
					Profile:    cfg.Profile,
					AuthMethod: cfg.AuthMethod,
					SSHPublicKey: func() string {
						return cfg.SSHPublicKey
					}(),
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
				return reused, nil
			}
		}
	}

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
	if opts.OnCreated != nil {
		opts.OnCreated(created)
	}
	waitTimeout := ActiveWaitTimeout
	if opts.WaitTimeout > 0 {
		waitTimeout = opts.WaitTimeout
	}
	active, err := WaitForActive(client, created.ID, waitTimeout, ActivePollIntervalSeconds, opts.OnPoll)
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
		ID:         metadata.BastionID,
		Region:     cfg.Region,
		Profile:    cfg.Profile,
		AuthMethod: cfg.AuthMethod,
		SSHPublicKey: func() string {
			return cfg.SSHPublicKey
		}(),
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

func listActiveSessions(client OCIClient, bastionID string) []SessionInfo {
	sessions, err := client.ListSessions(strings.TrimSpace(bastionID))
	if err != nil {
		return nil
	}
	return sessions
}

func selectReusableSession(metadata SessionMetadata, sessions []SessionInfo, now time.Time, minTTL time.Duration) (string, bool) {
	candidates := make([]SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		if !sessionInfoIsReusable(metadata, s, now, minTTL) {
			continue
		}
		candidates = append(candidates, s)
	}
	if len(candidates) == 0 {
		return "", false
	}
	slices.SortFunc(candidates, func(a, b SessionInfo) int {
		if !a.TimeExpires.Equal(b.TimeExpires) {
			if a.TimeExpires.After(b.TimeExpires) {
				return -1
			}
			return 1
		}
		if a.TimeCreated.Equal(b.TimeCreated) {
			return strings.Compare(a.ID, b.ID)
		}
		if a.TimeCreated.After(b.TimeCreated) {
			return -1
		}
		return 1
	})
	return candidates[0].ID, true
}

func sessionInfoIsReusable(metadata SessionMetadata, s SessionInfo, now time.Time, minTTL time.Duration) bool {
	if !strings.EqualFold(strings.TrimSpace(s.LifecycleState), "ACTIVE") {
		return false
	}
	if metadata.BastionID != "" && strings.TrimSpace(s.BastionID) != "" && strings.TrimSpace(s.BastionID) != strings.TrimSpace(metadata.BastionID) {
		return false
	}
	if metadata.InstanceID != "" && strings.TrimSpace(s.TargetResource) != "" && strings.TrimSpace(s.TargetResource) != strings.TrimSpace(metadata.InstanceID) {
		return false
	}
	if metadata.PrivateIP != "" && strings.TrimSpace(s.TargetPrivate) != "" && strings.TrimSpace(s.TargetPrivate) != strings.TrimSpace(metadata.PrivateIP) {
		return false
	}
	if !s.TimeExpires.IsZero() && !s.TimeExpires.After(now.Add(minTTL)) {
		return false
	}
	return true
}

func sessionIsReusable(metadata SessionMetadata, s BastionSession, now time.Time, minTTL time.Duration) bool {
	if !strings.EqualFold(strings.TrimSpace(s.LifecycleState), "ACTIVE") {
		return false
	}
	if metadata.BastionID != "" && strings.TrimSpace(s.BastionID) != "" && strings.TrimSpace(s.BastionID) != strings.TrimSpace(metadata.BastionID) {
		return false
	}
	if metadata.InstanceID != "" && strings.TrimSpace(s.TargetResourceID) != "" && strings.TrimSpace(s.TargetResourceID) != strings.TrimSpace(metadata.InstanceID) {
		return false
	}
	if metadata.PrivateIP != "" && strings.TrimSpace(s.TargetPrivateIP) != "" && strings.TrimSpace(s.TargetPrivateIP) != strings.TrimSpace(metadata.PrivateIP) {
		return false
	}
	if !s.TimeExpires.IsZero() && !s.TimeExpires.After(now.Add(minTTL)) {
		return false
	}
	return true
}

func resolveTargetMetadata(cfg Config, client OCIClient, opts RefreshOptions) (SessionMetadata, error) {
	metadata := SessionMetadata{
		BastionID:  strings.TrimSpace(opts.BastionID),
		InstanceID: strings.TrimSpace(opts.InstanceID),
		PrivateIP:  strings.TrimSpace(opts.PrivateIP),
	}
	if metadata.BastionID == "" || metadata.InstanceID == "" || metadata.PrivateIP == "" {
		if err := fillFromCachedAndLiveSession(cfg, client, &metadata); err != nil {
			return SessionMetadata{}, err
		}
	}
	if metadata.BastionID == "" || metadata.InstanceID == "" || metadata.PrivateIP == "" {
		if fromTF, err := LoadTargetDetails(cfg); err == nil {
			if metadata.BastionID == "" {
				metadata.BastionID = strings.TrimSpace(fromTF.BastionID)
			}
			if metadata.InstanceID == "" {
				metadata.InstanceID = strings.TrimSpace(fromTF.InstanceID)
			}
			if metadata.PrivateIP == "" {
				metadata.PrivateIP = strings.TrimSpace(fromTF.PrivateIP)
			}
		}
	}
	if metadata.BastionID == "" || metadata.InstanceID == "" || metadata.PrivateIP == "" {
		return SessionMetadata{}, fmt.Errorf("unable to resolve bastion target details; pass --instance-id and --private-ip, or ensure an existing session with target details")
	}
	return metadata, nil
}

func fillFromCachedAndLiveSession(cfg Config, client OCIClient, metadata *SessionMetadata) error {
	cached, err := LoadSession(cfg.SessionStatePath)
	if err != nil {
		return err
	}
	if cached != nil {
		applySessionMetadata(metadata, *cached)
		if metadata.InstanceID != "" && metadata.PrivateIP != "" && metadata.BastionID != "" {
			return nil
		}
		if strings.TrimSpace(cached.ID) != "" {
			if live, getErr := client.GetSession(strings.TrimSpace(cached.ID)); getErr == nil {
				applySessionMetadata(metadata, live)
				if metadata.InstanceID != "" && metadata.PrivateIP != "" && metadata.BastionID != "" {
					return nil
				}
			}
		}
	}
	if strings.TrimSpace(metadata.BastionID) == "" {
		return nil
	}
	sessions, err := client.ListSessions(strings.TrimSpace(metadata.BastionID))
	if err != nil {
		return nil
	}
	if len(sessions) == 0 {
		return nil
	}
	candidates := make([]SessionInfo, 0, len(sessions))
	for _, s := range sessions {
		if strings.TrimSpace(s.TargetResource) == "" || strings.TrimSpace(s.TargetPrivate) == "" {
			continue
		}
		candidates = append(candidates, s)
	}
	if len(candidates) == 0 {
		return nil
	}
	slices.SortFunc(candidates, func(a, b SessionInfo) int {
		if a.TimeCreated.Equal(b.TimeCreated) {
			return strings.Compare(a.ID, b.ID)
		}
		if a.TimeCreated.After(b.TimeCreated) {
			return -1
		}
		return 1
	})
	for _, s := range candidates {
		if strings.EqualFold(strings.TrimSpace(s.LifecycleState), "ACTIVE") {
			applySessionInfo(metadata, s)
			return nil
		}
	}
	applySessionInfo(metadata, candidates[0])
	return nil
}

func applySessionMetadata(metadata *SessionMetadata, session BastionSession) {
	sBastionID := strings.TrimSpace(session.BastionID)
	if sBastionID != "" {
		if metadata.BastionID == "" {
			metadata.BastionID = sBastionID
		}
		if metadata.BastionID != sBastionID {
			return
		}
	}
	if metadata.InstanceID == "" {
		metadata.InstanceID = strings.TrimSpace(session.TargetResourceID)
	}
	if metadata.PrivateIP == "" {
		metadata.PrivateIP = strings.TrimSpace(session.TargetPrivateIP)
	}
}

func applySessionInfo(metadata *SessionMetadata, session SessionInfo) {
	sBastionID := strings.TrimSpace(session.BastionID)
	if sBastionID != "" {
		if metadata.BastionID == "" {
			metadata.BastionID = sBastionID
		}
		if metadata.BastionID != sBastionID {
			return
		}
	}
	if metadata.InstanceID == "" {
		metadata.InstanceID = strings.TrimSpace(session.TargetResource)
	}
	if metadata.PrivateIP == "" {
		metadata.PrivateIP = strings.TrimSpace(session.TargetPrivate)
	}
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
			AuthMethod:    cfg.AuthMethod,
			SSHPublicKey:  cfg.SSHPublicKey,
			ContextName:   ctxName,
			LastSeenAt:    time.Now().UTC(),
		})
	}
	if len(tracked) > 0 {
		_ = UpsertTracked(cfg.TrackedBastionsPath, tracked...)
	}
	return items, nil
}
