package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ociContextConfig struct {
	Contexts       []ociContextEntry `yaml:"contexts" json:"contexts"`
	CurrentContext string            `yaml:"current_context" json:"current_context"`
}

type ociContextEntry struct {
	Name            string `yaml:"name" json:"name"`
	Profile         string `yaml:"profile" json:"profile"`
	TenancyOCID     string `yaml:"tenancy_ocid" json:"tenancy_ocid"`
	CompartmentOCID string `yaml:"compartment_ocid" json:"compartment_ocid"`
	Region          string `yaml:"region" json:"region"`
	User            string `yaml:"user" json:"user"`
}

func ResolveOCIContextConfigPath(explicit string, global bool) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	globalPath := filepath.Join(home, ".oci-context", "config.yml")
	if global {
		return globalPath, nil
	}
	wd, err := os.Getwd()
	if err != nil {
		return globalPath, nil
	}
	candidates := []string{
		".oci-context.yml",
		".oci-context.json",
		filepath.Join(".oci-context", "config.yml"),
		filepath.Join(".oci-context", "config.json"),
		"oci-context.yml",
		"oci-context.json",
		filepath.Join("oci-context", "config.yml"),
		filepath.Join("oci-context", "config.json"),
	}
	for _, rel := range candidates {
		p := filepath.Join(wd, rel)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return globalPath, nil
}

func LoadCurrentOCIContext(path string) (*ContextRef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := ociContextConfig{}
	switch filepath.Ext(path) {
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	default:
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, err
		}
	}
	if cfg.CurrentContext == "" {
		return nil, fmt.Errorf("oci-context has no current context set")
	}
	for _, c := range cfg.Contexts {
		if c.Name == cfg.CurrentContext {
			return &ContextRef{
				Name:            c.Name,
				Profile:         c.Profile,
				Region:          c.Region,
				CompartmentOCID: c.CompartmentOCID,
				TenancyOCID:     c.TenancyOCID,
				User:            c.User,
			}, nil
		}
	}
	return nil, fmt.Errorf("oci-context current context %q not found", cfg.CurrentContext)
}
