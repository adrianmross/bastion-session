package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type TrackedBastion struct {
	ID            string    `json:"id" yaml:"id"`
	Name          string    `json:"name" yaml:"name"`
	CompartmentID string    `json:"compartment_id" yaml:"compartment_id"`
	Region        string    `json:"region" yaml:"region"`
	Profile       string    `json:"profile" yaml:"profile"`
	AuthMethod    string    `json:"auth_method,omitempty" yaml:"auth_method,omitempty"`
	SSHPublicKey  string    `json:"ssh_public_key,omitempty" yaml:"ssh_public_key,omitempty"`
	ContextName   string    `json:"context_name,omitempty" yaml:"context_name,omitempty"`
	LastSeenAt    time.Time `json:"last_seen_at" yaml:"last_seen_at"`
}

type CurrentBastion struct {
	ID            string    `json:"id" yaml:"id"`
	Name          string    `json:"name" yaml:"name"`
	CompartmentID string    `json:"compartment_id" yaml:"compartment_id"`
	Region        string    `json:"region" yaml:"region"`
	Profile       string    `json:"profile" yaml:"profile"`
	AuthMethod    string    `json:"auth_method,omitempty" yaml:"auth_method,omitempty"`
	SSHPublicKey  string    `json:"ssh_public_key,omitempty" yaml:"ssh_public_key,omitempty"`
	ContextName   string    `json:"context_name,omitempty" yaml:"context_name,omitempty"`
	Source        string    `json:"source,omitempty" yaml:"source,omitempty"`
	SelectedAt    time.Time `json:"selected_at" yaml:"selected_at"`
}

type trackedStore struct {
	Bastions []TrackedBastion `json:"bastions"`
}

func LoadTracked(path string) ([]TrackedBastion, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	st := trackedStore{}
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, nil
	}
	sort.Slice(st.Bastions, func(i, j int) bool {
		return st.Bastions[i].LastSeenAt.After(st.Bastions[j].LastSeenAt)
	})
	return st.Bastions, nil
}

func SaveTracked(path string, bastions []TrackedBastion) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(trackedStore{Bastions: bastions}, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func UpsertTracked(path string, items ...TrackedBastion) error {
	existing, err := LoadTracked(path)
	if err != nil {
		return err
	}
	idx := map[string]int{}
	for i, b := range existing {
		idx[b.ID] = i
	}
	now := time.Now().UTC()
	for _, b := range items {
		if b.ID == "" {
			continue
		}
		if b.LastSeenAt.IsZero() {
			b.LastSeenAt = now
		}
		if i, ok := idx[b.ID]; ok {
			cur := existing[i]
			if b.Name != "" {
				cur.Name = b.Name
			}
			if b.CompartmentID != "" {
				cur.CompartmentID = b.CompartmentID
			}
			if b.Region != "" {
				cur.Region = b.Region
			}
			if b.Profile != "" {
				cur.Profile = b.Profile
			}
			if b.AuthMethod != "" {
				cur.AuthMethod = b.AuthMethod
			}
			if b.SSHPublicKey != "" {
				cur.SSHPublicKey = b.SSHPublicKey
			}
			if b.ContextName != "" {
				cur.ContextName = b.ContextName
			}
			cur.LastSeenAt = b.LastSeenAt
			existing[i] = cur
			continue
		}
		idx[b.ID] = len(existing)
		existing = append(existing, b)
	}
	sort.Slice(existing, func(i, j int) bool {
		return existing[i].LastSeenAt.After(existing[j].LastSeenAt)
	})
	return SaveTracked(path, existing)
}

func SaveCurrent(path string, cur CurrentBastion) error {
	if cur.SelectedAt.IsZero() {
		cur.SelectedAt = time.Now().UTC()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(cur, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadCurrent(path string) (*CurrentBastion, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cur CurrentBastion
	if err := json.Unmarshal(data, &cur); err != nil {
		return nil, nil
	}
	if cur.ID == "" {
		return nil, nil
	}
	return &cur, nil
}

func RemoveTracked(path string, ids ...string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	existing, err := LoadTracked(path)
	if err != nil {
		return 0, err
	}
	removeSet := map[string]bool{}
	for _, id := range ids {
		if id != "" {
			removeSet[id] = true
		}
	}
	kept := make([]TrackedBastion, 0, len(existing))
	removed := 0
	for _, b := range existing {
		if removeSet[b.ID] {
			removed++
			continue
		}
		kept = append(kept, b)
	}
	if err := SaveTracked(path, kept); err != nil {
		return 0, err
	}
	return removed, nil
}
