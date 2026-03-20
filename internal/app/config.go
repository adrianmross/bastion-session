package app

import (
	"os"
	"path/filepath"
)

const (
	DefaultProfile    = "oabcs1-terraform"
	DefaultRegion     = "us-chicago-1"
	DefaultAuthMethod = ""
	DefaultTargetUser = "opc"
)

type Config struct {
	Profile              string
	Region               string
	AuthMethod           string
	TargetUser           string
	SSHPublicKey         string
	SSHPrivateKey        string
	SSHIncludePath       string
	SessionStatePath     string
	TerraformOutputsPath string
	TrackedBastionsPath  string

	// OCI-context scoping
	OCIContextConfigPath string
	UseGlobalOCIContext  bool
	ContextScopeEnabled  bool
	ScopedContext        *ContextRef
}

type ContextRef struct {
	Name            string
	Profile         string
	AuthMethod      string
	Region          string
	CompartmentOCID string
	TenancyOCID     string
	User            string
}

func defaultPaths() (sshInclude, statePath, trackedPath string) {
	home, _ := os.UserHomeDir()
	sshInclude = filepath.Join(home, ".ssh", "config.d", "bastion-session")
	statePath = filepath.Join(home, ".cache", "bastion-session", "state.json")
	trackedPath = filepath.Join(home, ".cache", "bastion-session", "tracked-bastions.json")
	return
}

func ConfigFromEnv() Config {
	sshInclude, statePath, trackedPath := defaultPaths()
	cfg := Config{
		Profile:             DefaultProfile,
		Region:              DefaultRegion,
		AuthMethod:          DefaultAuthMethod,
		TargetUser:          DefaultTargetUser,
		SSHIncludePath:      sshInclude,
		SessionStatePath:    statePath,
		TrackedBastionsPath: trackedPath,
		ContextScopeEnabled: true,
	}
	if v := os.Getenv("PROFILE"); v != "" {
		cfg.Profile = v
	}
	if v := os.Getenv("REGION"); v != "" {
		cfg.Region = v
	}
	if v := os.Getenv("AUTH_METHOD"); v != "" {
		cfg.AuthMethod = v
	}
	if v := os.Getenv("TARGET_USER"); v != "" {
		cfg.TargetUser = v
	}
	if v := os.Getenv("SSH_PUBLIC_KEY"); v != "" {
		cfg.SSHPublicKey = v
	}
	if v := os.Getenv("SSH_PRIVATE_KEY"); v != "" {
		cfg.SSHPrivateKey = v
	}
	if v := os.Getenv("TERRAFORM_OUTPUTS"); v != "" {
		cfg.TerraformOutputsPath = v
	}
	if v := os.Getenv("BASTION_SESSION_STATE_PATH"); v != "" {
		cfg.SessionStatePath = v
	}
	if v := os.Getenv("BASTION_SESSION_SSH_INCLUDE"); v != "" {
		cfg.SSHIncludePath = v
	}
	if v := os.Getenv("BASTION_TRACKED_PATH"); v != "" {
		cfg.TrackedBastionsPath = v
	}
	if v := os.Getenv("BASTION_CONTEXT_SCOPE"); v == "0" || v == "false" {
		cfg.ContextScopeEnabled = false
	}
	return cfg
}

func (c *Config) ApplyContextScope(ctx *ContextRef) {
	if ctx == nil {
		return
	}
	c.ScopedContext = ctx
	if c.Profile == "" || c.Profile == DefaultProfile {
		if ctx.Profile != "" {
			c.Profile = ctx.Profile
		}
	}
	if c.Region == "" || c.Region == DefaultRegion {
		if ctx.Region != "" {
			c.Region = ctx.Region
		}
	}
	if c.AuthMethod == "" {
		if ctx.AuthMethod != "" {
			c.AuthMethod = ctx.AuthMethod
		}
	}
}
