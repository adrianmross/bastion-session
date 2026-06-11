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

const ociContextAuthEnsureTimeout = 90 * time.Second

func EnsureOCIContextAuth(cfg Config) error {
	if !cfg.ContextScopeEnabled || strings.TrimSpace(cfg.OCIContextName) == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), ociContextAuthEnsureTimeout)
	defer cancel()

	args := []string{"auth"}
	if strings.TrimSpace(cfg.OCIContextConfigPath) != "" {
		args = append(args, "--config", strings.TrimSpace(cfg.OCIContextConfigPath))
	}
	if cfg.UseGlobalOCIContext {
		args = append(args, "--global")
	}
	args = append(args,
		"--context", strings.TrimSpace(cfg.OCIContextName),
		"--no-interactive",
		"ensure",
		"--output", "json",
	)

	cmd := exec.CommandContext(ctx, resolveOCIContextBinary(), args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("oci-context auth ensure timed out for context %s", cfg.OCIContextName)
		}
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("oci-context auth ensure failed for context %s: %s", cfg.OCIContextName, detail)
	}
	return nil
}

func resolveOCIContextBinary() string {
	if bin := strings.TrimSpace(os.Getenv("OCI_CONTEXT_BIN")); bin != "" {
		return bin
	}
	if bin, err := exec.LookPath("oci-context"); err == nil {
		return bin
	}
	for _, candidate := range []string{
		"/opt/homebrew/bin/oci-context",
		"/usr/local/bin/oci-context",
		filepath.Join(os.Getenv("HOME"), ".local", "bin", "oci-context"),
	} {
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate
		}
	}
	return "oci-context"
}
