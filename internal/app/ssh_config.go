package app

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
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
