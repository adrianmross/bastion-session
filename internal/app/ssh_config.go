package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const sshConfigTimeout = 10 * time.Second

type SSHConfig struct {
	Host          string   `json:"host" yaml:"host"`
	HostName      string   `json:"hostname,omitempty" yaml:"hostname,omitempty"`
	User          string   `json:"user,omitempty" yaml:"user,omitempty"`
	ProxyJump     string   `json:"proxyjump,omitempty" yaml:"proxyjump,omitempty"`
	IdentityFile  string   `json:"identity_file,omitempty" yaml:"identity_file,omitempty"`
	IdentityFiles []string `json:"identity_files,omitempty" yaml:"identity_files,omitempty"`
	Raw           string   `json:"-" yaml:"-"`
	Error         string   `json:"error,omitempty" yaml:"error,omitempty"`
}

type SSHHostBlock struct {
	Path       string   `json:"path" yaml:"path"`
	Line       int      `json:"line" yaml:"line"`
	Patterns   []string `json:"patterns" yaml:"patterns"`
	ExactMatch bool     `json:"exact_match" yaml:"exact_match"`
}

type SSHConfigAudit struct {
	Host       string         `json:"host" yaml:"host"`
	Files      []string       `json:"files" yaml:"files"`
	Matches    []SSHHostBlock `json:"matches" yaml:"matches"`
	Competing  bool           `json:"competing" yaml:"competing"`
	Warning    string         `json:"warning,omitempty" yaml:"warning,omitempty"`
	ScanErrors []string       `json:"scan_errors,omitempty" yaml:"scan_errors,omitempty"`
}

func ReadSSHConfig(host string) (SSHConfig, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return SSHConfig{}, fmt.Errorf("host is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), sshConfigTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ssh", "-G", host)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := ParseSSHConfig(host, stdout.String())
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Sprintf("ssh -G timed out after %s", sshConfigTimeout)
		} else {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = strings.TrimSpace(stdout.String())
			}
			result.Error = strings.TrimSpace(fmt.Sprintf("%v: %s", err, msg))
		}
		return result, err
	}
	return result, nil
}

func ParseSSHConfig(host string, text string) SSHConfig {
	result := SSHConfig{Host: strings.TrimSpace(host), Raw: text}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.ToLower(fields[0])
		value := strings.TrimSpace(strings.TrimPrefix(line, fields[0]))
		value = strings.Trim(value, "\"")
		if strings.EqualFold(value, "none") || strings.EqualFold(value, "(none)") {
			value = ""
		}
		switch key {
		case "hostname":
			result.HostName = value
		case "user":
			result.User = value
		case "proxyjump":
			result.ProxyJump = value
		case "identityfile":
			if value != "" {
				result.IdentityFiles = append(result.IdentityFiles, value)
				if result.IdentityFile == "" {
					result.IdentityFile = value
				}
			}
		}
	}
	return result
}

func AuditSSHConfig(host string, extraPaths ...string) SSHConfigAudit {
	host = strings.TrimSpace(host)
	audit := SSHConfigAudit{Host: host}
	files := defaultSSHConfigScanPaths(extraPaths...)
	audit.Files = files
	exactMatches := 0
	for _, path := range files {
		blocks, err := scanSSHHostBlocks(path, host)
		if err != nil {
			audit.ScanErrors = append(audit.ScanErrors, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		for _, block := range blocks {
			if block.ExactMatch {
				exactMatches++
			}
			audit.Matches = append(audit.Matches, block)
		}
	}
	audit.Competing = exactMatches > 1
	if audit.Competing {
		audit.Warning = fmt.Sprintf("found %d exact Host blocks for %s", exactMatches, host)
	}
	return audit
}

func defaultSSHConfigScanPaths(extraPaths ...string) []string {
	seen := map[string]bool{}
	var paths []string
	add := func(path string) {
		path = strings.TrimSpace(expandHome(path))
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		add(filepath.Join(home, ".ssh", "config"))
		matches, _ := filepath.Glob(filepath.Join(home, ".ssh", "config.d", "*"))
		for _, match := range matches {
			add(match)
		}
	}
	for _, path := range extraPaths {
		add(path)
	}
	return paths
}

func scanSSHHostBlocks(path string, host string) ([]SSHHostBlock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	blocks := []SSHHostBlock{}
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 || !strings.EqualFold(fields[0], "Host") {
			continue
		}
		patterns := fields[1:]
		exact := false
		matches := false
		for _, pattern := range patterns {
			p := strings.TrimSpace(pattern)
			if p == host {
				exact = true
				matches = true
				continue
			}
			if ok, _ := filepath.Match(p, host); ok {
				matches = true
			}
		}
		if matches {
			blocks = append(blocks, SSHHostBlock{Path: path, Line: i + 1, Patterns: patterns, ExactMatch: exact})
		}
	}
	return blocks, nil
}

func expandHome(path string) string {
	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		if home == "" {
			return path
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return path
}
