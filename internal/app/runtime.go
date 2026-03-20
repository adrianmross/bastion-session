package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	ActivePollIntervalSeconds = 5 * time.Second
	ActiveWaitTimeout         = 2 * time.Minute
	DefaultWatchInterval      = 300 * time.Second
	MinAutoRefresh            = 30 * time.Second
	AutoRefreshMargin         = ActiveWaitTimeout + 30*time.Second
)

type SessionMetadata struct {
	BastionID   string
	InstanceID  string
	PrivateIP   string
	BastionHost string
}

var publicKeyEnvVars = []string{
	"SSH_PUBLIC_KEY",
	"TF_VAR_bastion_ssh_public_key_path",
	"TF_VAR_ssh_public_key_path",
	"BASTION_SSH_PUBLIC_KEY_PATH",
	"SSH_PUBLIC_KEY_PATH",
}

var publicKeyOutputKeys = []string{
	"bastion_ssh_public_key_path",
	"ssh_public_key_path",
	"public_key_path",
}

var tfvarsFilenames = []string{
	"env.tfvars",
	"terraform.tfvars",
	"terraform.tfvars.json",
}

var (
	tfvarsDoubleQuote = regexp.MustCompile(`^\s*([A-Za-z0-9_]+)\s*=\s*"([^"]+)"\s*(?:#.*)?$`)
	tfvarsSingleQuote = regexp.MustCompile(`^\s*([A-Za-z0-9_]+)\s*=\s*'([^']+)'\s*(?:#.*)?$`)
)

func ResolvePublicKey(cfg Config) string {
	seen := map[string]bool{}
	candidates := make([]string, 0, 32)
	add := func(raw, baseDir string) {
		if strings.TrimSpace(raw) == "" {
			return
		}
		p := strings.TrimSpace(raw)
		if strings.HasPrefix(p, "~") {
			home, _ := os.UserHomeDir()
			p = filepath.Join(home, strings.TrimPrefix(p, "~/"))
		}
		if !filepath.IsAbs(p) {
			if baseDir == "" {
				cwd, _ := os.Getwd()
				baseDir = cwd
			}
			p = filepath.Join(baseDir, p)
		}
		p = filepath.Clean(p)
		if !seen[p] {
			seen[p] = true
			candidates = append(candidates, p)
		}
	}

	add(cfg.SSHPublicKey, "")
	for _, envName := range publicKeyEnvVars {
		add(os.Getenv(envName), "")
	}

	if outPath := ResolveOutputsPath(cfg); outPath != "" {
		if outputs, err := ReadOutputs(outPath); err == nil {
			for _, k := range publicKeyOutputKeys {
				if v, ok := outputs[k]; ok {
					add(fmt.Sprintf("%v", v), filepath.Dir(outPath))
				}
			}
		}
		for _, name := range tfvarsFilenames {
			tfvarsPath := filepath.Join(filepath.Dir(outPath), name)
			for _, p := range extractPathsFromTFVars(tfvarsPath) {
				add(p, filepath.Dir(tfvarsPath))
			}
		}
	}

	cwd, _ := os.Getwd()
	for d := cwd; d != ""; d = filepath.Dir(d) {
		for _, name := range tfvarsFilenames {
			p := filepath.Join(d, name)
			for _, v := range extractPathsFromTFVars(p) {
				add(v, filepath.Dir(p))
			}
		}
		if parent := filepath.Dir(d); parent == d {
			break
		}
	}

	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func extractPathsFromTFVars(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if strings.HasSuffix(path, ".json") {
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err != nil {
			return nil
		}
		res := make([]string, 0, 4)
		for _, key := range publicKeyOutputKeys {
			if v, ok := obj[key].(string); ok && strings.TrimSpace(v) != "" {
				res = append(res, strings.TrimSpace(v))
			}
		}
		return res
	}

	s := bufio.NewScanner(strings.NewReader(string(data)))
	res := []string{}
	for s.Scan() {
		line := s.Text()
		m := tfvarsDoubleQuote.FindStringSubmatch(line)
		if m == nil {
			m = tfvarsSingleQuote.FindStringSubmatch(line)
		}
		if len(m) != 3 {
			continue
		}
		key := m[1]
		val := strings.TrimSpace(m[2])
		for _, allowed := range publicKeyOutputKeys {
			if key == allowed && val != "" {
				res = append(res, val)
			}
		}
	}
	return res
}

func EnsureSSHInclude(includePath string) error {
	if err := os.MkdirAll(filepath.Dir(includePath), 0o755); err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	mainConfig := filepath.Join(home, ".ssh", "config")
	if _, err := os.Stat(mainConfig); err != nil {
		return nil
	}
	includeLine := "Include " + includePath
	data, err := os.ReadFile(mainConfig)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	for _, ln := range lines {
		if strings.TrimSpace(ln) == includeLine {
			return nil
		}
	}
	content := strings.TrimRight(string(data), "\n") + "\n" + includeLine + "\n"
	return os.WriteFile(mainConfig, []byte(content), 0o600)
}

func UpdateSSHFragment(cfg Config, sessionID string) error {
	privateKey := cfg.SSHPrivateKey
	if privateKey == "" && strings.HasSuffix(cfg.SSHPublicKey, ".pub") {
		candidate := strings.TrimSuffix(cfg.SSHPublicKey, ".pub")
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			privateKey = candidate
		}
	}
	lines := []string{
		"# Managed by bastion-session CLI",
		fmt.Sprintf("Host %s-bastion", cfg.Profile),
		fmt.Sprintf("  HostName host.bastion.%s.oci.oraclecloud.com", cfg.Region),
		"  Port 22",
		fmt.Sprintf("  User %s", sessionID),
	}
	if privateKey != "" {
		lines = append(lines, fmt.Sprintf("  IdentityFile %s", privateKey))
	}
	lines = append(lines,
		"  IdentitiesOnly yes",
		"  IdentityAgent none",
	)
	content := strings.Join(lines, "\n") + "\n"
	if err := os.MkdirAll(filepath.Dir(cfg.SSHIncludePath), 0o755); err != nil {
		return err
	}
	tmp := cfg.SSHIncludePath + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, cfg.SSHIncludePath)
}

func WaitForActive(client OCIClient, sessionID string, timeout time.Duration, poll time.Duration) (BastionSession, error) {
	deadline := time.Now().Add(timeout)
	for {
		s, err := client.GetSession(sessionID)
		if err != nil {
			return BastionSession{}, err
		}
		if strings.EqualFold(s.LifecycleState, "ACTIVE") {
			return s, nil
		}
		if time.Now().After(deadline) {
			return BastionSession{}, fmt.Errorf("session %s did not reach ACTIVE state (last state: %s)", sessionID, s.LifecycleState)
		}
		time.Sleep(poll)
	}
}

func AutoRefreshInterval(s BastionSession) time.Duration {
	ttl := s.ExpiresIn()
	if ttl < 0 {
		ttl = 0
	}
	interval := ttl - AutoRefreshMargin
	if interval < MinAutoRefresh {
		interval = MinAutoRefresh
	}
	return interval
}
