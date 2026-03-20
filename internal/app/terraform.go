package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func ReadOutputs(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	if out, ok := payload["outputs"].(map[string]any); ok {
		payload = out
	}
	result := make(map[string]any, len(payload))
	for k, v := range payload {
		if mv, ok := v.(map[string]any); ok {
			if vv, has := mv["value"]; has {
				result[k] = vv
				continue
			}
		}
		result[k] = v
	}
	return result, nil
}

func ResolveOutputsPath(cfg Config) string {
	seen := map[string]bool{}
	candidates := make([]string, 0, 32)
	add := func(p string) {
		if p == "" {
			return
		}
		ap, err := filepath.Abs(p)
		if err != nil {
			ap = p
		}
		if !seen[ap] {
			seen[ap] = true
			candidates = append(candidates, ap)
		}
	}

	add(cfg.TerraformOutputsPath)
	cwd, _ := os.Getwd()
	for d := cwd; d != ""; d = filepath.Dir(d) {
		add(filepath.Join(d, "terraform.tfstate"))
		add(filepath.Join(d, "terraform.tfstate.json"))
		add(filepath.Join(d, "outputs.json"))
		if parent := filepath.Dir(d); parent == d {
			break
		}
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

func LoadTargetDetails(cfg Config) (SessionMetadata, error) {
	outPath := ResolveOutputsPath(cfg)
	if outPath == "" {
		return SessionMetadata{}, fmt.Errorf("unable to locate Terraform outputs; set TERRAFORM_OUTPUTS or provide terraform.tfstate")
	}
	outputs, err := ReadOutputs(outPath)
	if err != nil {
		return SessionMetadata{}, err
	}
	required := []string{"bastion_id", "instance_id", "private_ip"}
	for _, key := range required {
		if _, ok := outputs[key]; !ok {
			return SessionMetadata{}, fmt.Errorf("missing output %q in %s", key, outPath)
		}
	}
	return SessionMetadata{
		BastionID:  fmt.Sprintf("%v", outputs["bastion_id"]),
		InstanceID: fmt.Sprintf("%v", outputs["instance_id"]),
		PrivateIP:  fmt.Sprintf("%v", outputs["private_ip"]),
		BastionHost: func() string {
			if v, ok := outputs["bastion_public_ip"]; ok {
				return fmt.Sprintf("%v", v)
			}
			return ""
		}(),
	}, nil
}
