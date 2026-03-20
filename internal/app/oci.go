package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const ociCommandTimeout = 60 * time.Second

type TargetDetails struct {
	BastionID     string
	InstanceID    string
	PrivateIP     string
	TargetUser    string
	PublicKeyPath string
}

type BastionInfo struct {
	ID             string    `json:"id" yaml:"id"`
	Name           string    `json:"name" yaml:"name"`
	CompartmentID  string    `json:"compartment_id" yaml:"compartment_id"`
	LifecycleState string    `json:"lifecycle_state" yaml:"lifecycle_state"`
	TargetSubnetID string    `json:"target_subnet_id" yaml:"target_subnet_id"`
	DnsProxyStatus string    `json:"dns_proxy_status" yaml:"dns_proxy_status"`
	MaxSessionTTL  int       `json:"max_session_ttl_seconds" yaml:"max_session_ttl_seconds"`
	TimeCreated    time.Time `json:"time_created" yaml:"time_created"`
	Region         string    `json:"region" yaml:"region"`
	Profile        string    `json:"profile" yaml:"profile"`
	ScopeContext   string    `json:"scope_context,omitempty" yaml:"scope_context,omitempty"`
}

type OCIClient struct {
	Profile    string
	Region     string
	AuthMethod string
}

func (c OCIClient) run(args ...string) ([]byte, error) {
	cmdArgs := []string{"--profile", c.Profile}
	if c.Region != "" {
		cmdArgs = append(cmdArgs, "--region", c.Region)
	}
	if c.AuthMethod != "" {
		cmdArgs = append(cmdArgs, "--auth", c.AuthMethod)
	}
	cmdArgs = append(cmdArgs, args...)
	ctx, cancel := context.WithTimeout(context.Background(), ociCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "oci", cmdArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			if strings.EqualFold(strings.TrimSpace(c.AuthMethod), "security_token") {
				return nil, fmt.Errorf("timed out waiting for OCI CLI response; run `oci session authenticate --profile %s` to refresh the token", c.Profile)
			}
			return nil, fmt.Errorf("timed out waiting for OCI CLI response (profile=%s region=%s)", c.Profile, c.Region)
		}
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText == "" {
			stderrText = strings.TrimSpace(stdout.String())
		}
		msg := strings.ToLower(stderrText)
		if strings.Contains(msg, "security token") || strings.Contains(msg, "security_token") || strings.Contains(msg, "security-token") {
			return nil, fmt.Errorf("OCI CLI reported a security token authentication failure. Re-authenticate with `oci session authenticate --profile %s`", c.Profile)
		}
		return nil, fmt.Errorf("oci command failed: %w: %s", err, stderrText)
	}
	return stdout.Bytes(), nil
}

func (c OCIClient) CreateSession(target TargetDetails) (BastionSession, error) {
	out, err := c.run(
		"bastion", "session", "create-managed-ssh",
		"--bastion-id", target.BastionID,
		"--target-resource-id", target.InstanceID,
		"--target-private-ip", target.PrivateIP,
		"--target-os-username", target.TargetUser,
		"--ssh-public-key-file", target.PublicKeyPath,
		"--query", "data",
		"--raw-output",
	)
	if err != nil {
		return BastionSession{}, err
	}
	return parseSessionJSON(out)
}

func (c OCIClient) GetSession(sessionID string) (BastionSession, error) {
	out, err := c.run(
		"bastion", "session", "get",
		"--session-id", sessionID,
		"--query", "data",
		"--raw-output",
	)
	if err != nil {
		return BastionSession{}, err
	}
	return parseSessionJSON(out)
}

func (c OCIClient) ListBastions(compartmentID string) ([]BastionInfo, error) {
	args := []string{"bastion", "bastion", "list", "--query", "data", "--raw-output"}
	if compartmentID != "" {
		args = append(args, "--compartment-id", compartmentID)
	}
	out, err := c.run(args...)
	if err != nil {
		return nil, err
	}
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return []BastionInfo{}, nil
	}
	var rows []map[string]any
	if err := json.Unmarshal(out, &rows); err != nil {
		const maxPreview = 256
		preview := string(out)
		if len(preview) > maxPreview {
			preview = preview[:maxPreview] + "..."
		}
		return nil, fmt.Errorf("failed to parse OCI bastion list output as JSON: %w (output=%q)", err, preview)
	}
	result := make([]BastionInfo, 0, len(rows))
	for _, row := range rows {
		bi := BastionInfo{
			ID:             asString(row, "id"),
			Name:           asString(row, "name"),
			CompartmentID:  asString(row, "compartmentId", "compartment-id", "compartment_id"),
			LifecycleState: asString(row, "lifecycleState", "lifecycle-state", "lifecycle_state"),
			TargetSubnetID: asString(row, "targetSubnetId", "target-subnet-id", "target_subnet_id"),
			DnsProxyStatus: asString(row, "dnsProxyStatus", "dns-proxy-status", "dns_proxy_status"),
			MaxSessionTTL:  asInt(row, "maxSessionTtlInSeconds", "max-session-ttl-in-seconds", "max_session_ttl_in_seconds"),
			Profile:        c.Profile,
			Region:         c.Region,
		}
		if t := asString(row, "timeCreated", "time-created", "time_created"); t != "" {
			if ts, err := time.Parse(time.RFC3339, t); err == nil {
				bi.TimeCreated = ts
			}
		}
		result = append(result, bi)
	}
	return result, nil
}

func parseSessionJSON(out []byte) (BastionSession, error) {
	var data map[string]any
	if err := json.Unmarshal(out, &data); err != nil {
		return BastionSession{}, err
	}
	createdRaw := asString(data, "timeCreated", "time-created", "time_created")
	created, err := parseTime(createdRaw)
	if err != nil {
		return BastionSession{}, err
	}
	expiresRaw := asString(data, "timeExpires", "time-expires", "time_expires")
	expires, err := computeExpiry(expiresRaw, data, created)
	if err != nil {
		return BastionSession{}, err
	}
	return BastionSession{
		ID:             asString(data, "id"),
		LifecycleState: asString(data, "lifecycleState", "lifecycle-state", "lifecycle_state"),
		TimeCreated:    created,
		TimeExpires:    expires,
	}, nil
}

func parseTime(v string) (time.Time, error) {
	if v == "" {
		return time.Time{}, fmt.Errorf("missing time value")
	}
	t, err := time.Parse(time.RFC3339, v)
	if err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339Nano, v)
}

func computeExpiry(explicit string, row map[string]any, created time.Time) (time.Time, error) {
	if explicit != "" {
		return parseTime(explicit)
	}
	ttl := asInt(row, "sessionTtlInSeconds", "session-ttl-in-seconds", "session_ttl_in_seconds")
	if ttl > 0 {
		return created.Add(time.Duration(ttl) * time.Second), nil
	}
	return created.Add(time.Hour), nil
}

func asString(row map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := row[k]; ok {
			switch vv := v.(type) {
			case string:
				return vv
			default:
				return fmt.Sprintf("%v", vv)
			}
		}
	}
	return ""
}

func asInt(row map[string]any, keys ...string) int {
	for _, k := range keys {
		if v, ok := row[k]; ok {
			switch vv := v.(type) {
			case float64:
				return int(vv)
			case int:
				return vv
			case string:
				i, _ := strconv.Atoi(vv)
				return i
			}
		}
	}
	return 0
}
