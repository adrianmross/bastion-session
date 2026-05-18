package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type TrackedTarget struct {
	Name                 string    `json:"name" yaml:"name"`
	InstanceID           string    `json:"instance_id" yaml:"instance_id"`
	PrivateIP            string    `json:"private_ip" yaml:"private_ip"`
	User                 string    `json:"user,omitempty" yaml:"user,omitempty"`
	IdentityFile         string    `json:"identity_file,omitempty" yaml:"identity_file,omitempty"`
	BastionID            string    `json:"bastion_id,omitempty" yaml:"bastion_id,omitempty"`
	TerraformOutputsPath string    `json:"terraform_outputs,omitempty" yaml:"terraform_outputs,omitempty"`
	LastSeenAt           time.Time `json:"last_seen_at" yaml:"last_seen_at"`
}

type trackedTargetStore struct {
	Targets []TrackedTarget `json:"targets"`
}

func LoadTrackedTargets(path string) ([]TrackedTarget, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	st := trackedTargetStore{}
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, nil
	}
	sortTrackedTargets(st.Targets)
	return st.Targets, nil
}

func SaveTrackedTargets(path string, targets []TrackedTarget) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	sortTrackedTargets(targets)
	buf, err := json.MarshalIndent(trackedTargetStore{Targets: targets}, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func UpsertTrackedTarget(path string, target TrackedTarget) error {
	target.Name = strings.TrimSpace(target.Name)
	if target.Name == "" {
		return fmt.Errorf("target name is required")
	}
	existing, err := LoadTrackedTargets(path)
	if err != nil {
		return err
	}
	if target.LastSeenAt.IsZero() {
		target.LastSeenAt = time.Now().UTC()
	}
	for i, cur := range existing {
		if cur.Name != target.Name {
			continue
		}
		existing[i] = mergeTrackedTarget(cur, target)
		return SaveTrackedTargets(path, existing)
	}
	existing = append(existing, target)
	return SaveTrackedTargets(path, existing)
}

func FindTrackedTarget(path string, name string) (*TrackedTarget, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, nil
	}
	targets, err := LoadTrackedTargets(path)
	if err != nil {
		return nil, err
	}
	for _, target := range targets {
		if target.Name == name {
			cp := target
			return &cp, nil
		}
	}
	return nil, nil
}

func RemoveTrackedTarget(path string, names ...string) (int, error) {
	if len(names) == 0 {
		return 0, nil
	}
	targets, err := LoadTrackedTargets(path)
	if err != nil {
		return 0, err
	}
	removeSet := map[string]bool{}
	for _, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			removeSet[name] = true
		}
	}
	kept := make([]TrackedTarget, 0, len(targets))
	removed := 0
	for _, target := range targets {
		if removeSet[target.Name] {
			removed++
			continue
		}
		kept = append(kept, target)
	}
	if err := SaveTrackedTargets(path, kept); err != nil {
		return 0, err
	}
	return removed, nil
}

func mergeTrackedTarget(cur, next TrackedTarget) TrackedTarget {
	if next.InstanceID != "" {
		cur.InstanceID = next.InstanceID
	}
	if next.PrivateIP != "" {
		cur.PrivateIP = next.PrivateIP
	}
	if next.User != "" {
		cur.User = next.User
	}
	if next.IdentityFile != "" {
		cur.IdentityFile = next.IdentityFile
	}
	if next.BastionID != "" {
		cur.BastionID = next.BastionID
	}
	if next.TerraformOutputsPath != "" {
		cur.TerraformOutputsPath = next.TerraformOutputsPath
	}
	if !next.LastSeenAt.IsZero() {
		cur.LastSeenAt = next.LastSeenAt
	}
	return cur
}

func sortTrackedTargets(targets []TrackedTarget) {
	sort.Slice(targets, func(i, j int) bool {
		if !targets[i].LastSeenAt.Equal(targets[j].LastSeenAt) {
			return targets[i].LastSeenAt.After(targets[j].LastSeenAt)
		}
		return targets[i].Name < targets[j].Name
	})
}
