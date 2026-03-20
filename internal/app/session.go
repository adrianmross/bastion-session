package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type BastionSession struct {
	ID             string    `json:"id" yaml:"id"`
	LifecycleState string    `json:"lifecycle_state" yaml:"lifecycle_state"`
	TimeCreated    time.Time `json:"time_created" yaml:"time_created"`
	TimeExpires    time.Time `json:"time_expires" yaml:"time_expires"`
}

func (s BastionSession) ExpiresIn() time.Duration {
	return time.Until(s.TimeExpires)
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
