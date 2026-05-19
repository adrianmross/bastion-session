package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BastionSession struct {
	ID               string    `json:"id" yaml:"id"`
	BastionID        string    `json:"bastion_id,omitempty" yaml:"bastion_id,omitempty"`
	TargetResourceID string    `json:"target_resource_id,omitempty" yaml:"target_resource_id,omitempty"`
	TargetPrivateIP  string    `json:"target_private_ip,omitempty" yaml:"target_private_ip,omitempty"`
	LifecycleState   string    `json:"lifecycle_state" yaml:"lifecycle_state"`
	TimeCreated      time.Time `json:"time_created" yaml:"time_created"`
	TimeExpires      time.Time `json:"time_expires" yaml:"time_expires"`
}

func (s BastionSession) ExpiresIn() time.Duration {
	return time.Until(s.TimeExpires)
}

const NearExpiryWarningTTL = 15 * time.Minute

func SessionExpiryWarning(expires time.Time, now time.Time) string {
	if expires.IsZero() || !expires.After(now) {
		return ""
	}
	remaining := expires.Sub(now)
	if remaining > NearExpiryWarningTTL {
		return ""
	}
	return "session expires in " + remaining.Round(time.Second).String()
}

type sessionState struct {
	Session *BastionSession `json:"session,omitempty"`
}

func LoadSession(path string) (*BastionSession, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	st := sessionState{}
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, nil
	}
	return st.Session, nil
}

func SaveSession(path string, s BastionSession) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(sessionState{Session: &s}, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

type SessionPruneResult struct {
	StatePath string `json:"state_path" yaml:"state_path"`
	Pruned    bool   `json:"pruned" yaml:"pruned"`
	Reason    string `json:"reason" yaml:"reason"`
	SessionID string `json:"session_id,omitempty" yaml:"session_id,omitempty"`
	Expires   string `json:"expires,omitempty" yaml:"expires,omitempty"`
}

func PruneExpiredSession(path string, now time.Time) (SessionPruneResult, error) {
	result := SessionPruneResult{StatePath: path, Reason: "no cached session"}
	s, err := LoadSession(path)
	if err != nil {
		return result, err
	}
	if s == nil {
		return result, nil
	}
	result.SessionID = strings.TrimSpace(s.ID)
	if !s.TimeExpires.IsZero() {
		result.Expires = s.TimeExpires.Format(time.RFC3339)
	}
	if s.TimeExpires.IsZero() || s.TimeExpires.After(now) {
		result.Reason = "cached session is not expired"
		return result, nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return result, err
	}
	result.Pruned = true
	result.Reason = "expired cached session removed"
	return result, nil
}
