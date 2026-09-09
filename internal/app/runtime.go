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

	"golang.org/x/crypto/ssh"
)

const (
	ActivePollIntervalSeconds = 5 * time.Second
	ActiveWaitTimeout         = 2 * time.Minute
	DefaultWatchInterval      = 300 * time.Second
	MinAutoRefresh            = 30 * time.Second
	AutoRefreshMargin         = ActiveWaitTimeout + 30*time.Second

	// A session reaches ACTIVE before its SSH front door will accept the session key.
	// Measured against a live bastion (us-sanjose-1, 4 runs, port-forwarding sessions):
	// the first connection attempt after ACTIVE failed EVERY time, and the forward was
	// provable 4-9s later. The failure surfaces as "Permission denied (publickey)",
	// which reads like a key problem and sends you looking in the wrong place.
	SSHReadyTimeout      = 60 * time.Second
	SSHReadyPollInterval = 3 * time.Second
)

type SessionMetadata struct {
	BastionID   string
	InstanceID  string
	PrivateIP   string
	BastionHost string
}

type TargetSSHHost struct {
	Alias        string
	HostName     string
	User         string
	IdentityFile string
	ProxyJump    string
}

type SSHHostEntry struct {
	Alias         string
	Profile       string
	Region        string
	SessionID     string
	SSHPublicKey  string
	SSHPrivateKey string
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
	home, _ := os.UserHomeDir()
	if home != "" {
		add(filepath.Join(home, ".ssh", "id_ed25519.pub"), "")
		add(filepath.Join(home, ".ssh", "id_rsa.pub"), "")
		add(filepath.Join(home, ".ssh", "id_ecdsa.pub"), "")
		add(filepath.Join(home, ".ssh", "id_dsa.pub"), "")
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

// identityHardeningLines pins which identity ssh offers for a generated Host block.
//
// `IdentitiesOnly yes` is always safe: it stops ssh walking the agent and offering
// unrelated keys first, which on a well-stocked agent can exhaust MaxAuthTries before the
// right key is ever tried.
//
// `IdentityAgent none` is NOT always safe, and used to be emitted unconditionally. It
// makes ssh ignore the agent entirely, so a PASSPHRASE-PROTECTED key -- whose usable
// decrypted copy lives only in the agent -- can no longer authenticate: every connection
// either prompts for the passphrase or, non-interactively, fails outright with
// "Permission denied (publickey)". Verified against a live bastion: with the agent, the
// forward works; with `IdentityAgent none`, the identical session is denied.
//
// So it is emitted only when the private key is readable WITHOUT a passphrase, where it
// is pure hardening and costs nothing.
func identityHardeningLines(privateKey string) []string {
	lines := []string{"  IdentitiesOnly yes"}
	if privateKeyIsPassphraseless(privateKey) {
		lines = append(lines, "  IdentityAgent none")
	}
	return lines
}

// privateKeyIsPassphraseless reports whether the key at path can be read without a
// passphrase. An unreadable or unknown key returns false, which keeps the agent available
// -- the safe direction, since disabling it is what breaks authentication.
func privateKeyIsPassphraseless(path string) bool {
	p := strings.TrimSpace(path)
	if p == "" {
		return false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	_, err = ssh.ParseRawPrivateKey(data)
	return err == nil
}

func UpdateSSHFragment(cfg Config, sessionID string) error {
	return UpdateSSHFragmentWithTarget(cfg, sessionID, TargetSSHHost{})
}

func UpdateSSHFragmentWithTarget(cfg Config, sessionID string, target TargetSSHHost) error {
	bastionAlias := cfg.Profile + "-bastion"
	lines := []string{
		"# Managed by bastion-session CLI",
		fmt.Sprintf("Host %s", bastionAlias),
		fmt.Sprintf("  HostName host.bastion.%s.oci.oraclecloud.com", cfg.Region),
		"  Port 22",
		fmt.Sprintf("  User %s", sessionID),
	}
	privateKey := resolvePrivateKey(cfg.SSHPrivateKey, cfg.SSHPublicKey)
	if privateKey != "" {
		lines = append(lines, fmt.Sprintf("  IdentityFile %s", privateKey))
	}
	lines = append(lines, identityHardeningLines(privateKey)...)
	if strings.TrimSpace(target.Alias) != "" {
		proxyJump := strings.TrimSpace(target.ProxyJump)
		if proxyJump == "" {
			proxyJump = bastionAlias
		}
		hostName := strings.TrimSpace(target.HostName)
		if hostName == "" {
			hostName = strings.TrimSpace(target.Alias)
		}
		user := strings.TrimSpace(target.User)
		if user == "" {
			user = cfg.TargetUser
		}
		lines = append(lines,
			"",
			fmt.Sprintf("Host %s", strings.TrimSpace(target.Alias)),
			fmt.Sprintf("  HostName %s", hostName),
			"  Port 22",
			fmt.Sprintf("  User %s", user),
		)
		if strings.TrimSpace(target.IdentityFile) != "" {
			lines = append(lines, fmt.Sprintf("  IdentityFile %s", strings.TrimSpace(target.IdentityFile)))
		}
		lines = append(lines,
			"  IdentitiesOnly yes",
			fmt.Sprintf("  ProxyJump %s", proxyJump),
		)
	}
	for _, block := range preservedTargetSSHBlocks(cfg.SSHIncludePath, bastionAlias, strings.TrimSpace(target.Alias)) {
		lines = append(lines, "")
		lines = append(lines, block...)
	}
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

func UpdateSSHFragmentForHosts(path string, hosts []SSHHostEntry) error {
	lines := []string{"# Managed by bastion-session CLI"}
	generatedAliases := map[string]bool{}
	for _, host := range hosts {
		alias := strings.TrimSpace(host.Alias)
		if alias == "" || strings.TrimSpace(host.Region) == "" || strings.TrimSpace(host.SessionID) == "" {
			continue
		}
		generatedAliases[alias] = true
		privateKey := resolvePrivateKey(host.SSHPrivateKey, host.SSHPublicKey)
		lines = append(lines,
			"",
			fmt.Sprintf("Host %s", alias),
			fmt.Sprintf("  HostName host.bastion.%s.oci.oraclecloud.com", host.Region),
			"  Port 22",
			fmt.Sprintf("  User %s", host.SessionID),
		)
		if privateKey != "" {
			lines = append(lines, fmt.Sprintf("  IdentityFile %s", privateKey))
		}
		lines = append(lines, identityHardeningLines(privateKey)...)
	}
	for _, block := range preservedNonBastionSSHBlocks(path, generatedAliases) {
		lines = append(lines, "")
		lines = append(lines, block...)
	}
	content := strings.Join(lines, "\n") + "\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func preservedNonBastionSSHBlocks(includePath string, generatedAliases map[string]bool) [][]string {
	data, err := os.ReadFile(includePath)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	var blocks [][]string
	var current []string
	flush := func() {
		if len(current) == 0 {
			return
		}
		hostLine := strings.TrimSpace(current[0])
		fields := strings.Fields(hostLine)
		if len(fields) < 2 || !strings.EqualFold(fields[0], "Host") {
			current = nil
			return
		}
		for _, alias := range fields[1:] {
			if generatedAliases[alias] {
				current = nil
				return
			}
		}
		for _, line := range current[1:] {
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), "hostname host.bastion.") {
				current = nil
				return
			}
		}
		blocks = append(blocks, current)
		current = nil
	}
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "Host ") {
			flush()
			current = []string{line}
			continue
		}
		if current != nil && strings.TrimSpace(line) != "" {
			current = append(current, line)
		}
	}
	flush()
	return blocks
}

func resolvePrivateKey(explicit, public string) string {
	if explicit != "" {
		return explicit
	}
	if strings.HasSuffix(public, ".pub") {
		candidate := strings.TrimSuffix(public, ".pub")
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
	}
	return ""
}

func preservedTargetSSHBlocks(includePath, bastionAlias, replacedAlias string) [][]string {
	data, err := os.ReadFile(includePath)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	var blocks [][]string
	var current []string
	flush := func() {
		if len(current) == 0 {
			return
		}
		hostLine := strings.TrimSpace(current[0])
		fields := strings.Fields(hostLine)
		if len(fields) >= 2 && strings.EqualFold(fields[0], "Host") {
			aliases := fields[1:]
			isBastion := false
			isReplaced := false
			for _, alias := range aliases {
				if alias == bastionAlias {
					isBastion = true
				}
				if replacedAlias != "" && alias == replacedAlias {
					isReplaced = true
				}
			}
			if !isBastion && !isReplaced {
				blocks = append(blocks, current)
			}
		}
		current = nil
	}
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "Host ") {
			flush()
			current = []string{line}
			continue
		}
		if current != nil && strings.TrimSpace(line) != "" {
			current = append(current, line)
		}
	}
	flush()
	return blocks
}

func WaitForActive(client OCIClient, sessionID string, timeout time.Duration, poll time.Duration, onPoll func(BastionSession)) (BastionSession, error) {
	deadline := time.Now().Add(timeout)
	lastState := ""
	for {
		s, err := client.GetSession(sessionID)
		if err != nil {
			return BastionSession{}, err
		}
		lastState = s.LifecycleState
		if onPoll != nil {
			onPoll(s)
		}
		if strings.EqualFold(s.LifecycleState, "ACTIVE") {
			return s, nil
		}
		if time.Now().After(deadline) {
			return BastionSession{}, fmt.Errorf("session %s did not reach ACTIVE state within %s (last state: %s)", sessionID, timeout.String(), lastState)
		}
		time.Sleep(poll)
	}
}

// SSHProbe reports whether the bastion's SSH front door will authenticate sessionID yet.
// Split out so callers can supply a fake in tests and so the probe can be reused by any
// session type -- both managed-SSH and port-forwarding authenticate at the same door.
type SSHProbe func(sessionID string) bool

// WaitForSSHReady blocks until probe succeeds, or timeout elapses.
//
// ACTIVE is necessary but NOT sufficient: see SSHReadyTimeout. Returning as soon as the
// lifecycle state flips hands the caller a session that reliably rejects the first
// connection. A nil probe makes this a no-op so existing callers keep their behaviour
// until they opt in.
func WaitForSSHReady(sessionID string, probe SSHProbe, timeout, poll time.Duration, onAttempt func(int)) error {
	if probe == nil {
		return nil
	}
	if poll <= 0 {
		poll = SSHReadyPollInterval
	}
	deadline := time.Now().Add(timeout)
	for attempt := 1; ; attempt++ {
		if onAttempt != nil {
			onAttempt(attempt)
		}
		if probe(sessionID) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("session %s reached ACTIVE but its SSH endpoint did not accept the key within %s", sessionID, timeout.String())
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
