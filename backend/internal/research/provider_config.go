package research

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const providerKeyEnv = "TRACKER_OPENCODE_PROVIDER_KEY"

// Import only the selected provider, never home agents, plugins, permissions or
// unrelated credentials. Keep it outside the model's evidence workspace.
func (r Runner) copyLocalProvider(home string) (string, error) {
	paths := []string{}
	if r.Config.OpenCodeConfigFile != "" {
		paths = append(paths, r.Config.OpenCodeConfigFile)
	} else {
		paths = append(paths, filepath.Join(userHome(), ".opencode", "opencode.json"))
	}
	providerID := strings.SplitN(r.Config.ResearchModel, "/", 2)[0]
	var selected map[string]any
	for _, path := range paths {
		file, err := os.Open(path)
		if os.IsNotExist(err) && r.Config.OpenCodeConfigFile == "" {
			continue
		}
		if err != nil {
			return "", errors.New("OpenCode provider configuration could not be read")
		}
		data, err := io.ReadAll(io.LimitReader(file, 1024*1024+1))
		file.Close()
		if err != nil || len(data) > 1024*1024 {
			return "", errors.New("OpenCode provider configuration could not be read or is oversized")
		}
		var cfg struct {
			Provider map[string]map[string]any `json:"provider"`
		}
		if json.Unmarshal(data, &cfg) != nil {
			return "", errors.New("OpenCode provider configuration must be valid JSON")
		}
		if provider, ok := cfg.Provider[providerID]; ok {
			selected = provider
		}
	}
	if selected == nil {
		return "", nil
	}
	// Only provider settings that OpenCode needs to identify and call the model.
	provider := map[string]any{}
	for _, name := range []string{"npm", "name", "options", "models", "whitelist", "blacklist", "env"} {
		if value, ok := selected[name]; ok {
			provider[name] = value
		}
	}
	key := ""
	if options, ok := provider["options"].(map[string]any); ok {
		if value, ok := options["apiKey"].(string); ok {
			key = value
			if strings.HasPrefix(value, "{env:") && strings.HasSuffix(value, "}") {
				key = os.Getenv(strings.TrimSuffix(strings.TrimPrefix(value, "{env:"), "}"))
			}
			if strings.HasPrefix(key, "{file:") {
				return "", errors.New("use a saved OpenCode credential, literal apiKey or {env:NAME} for the selected provider")
			}
			options["apiKey"] = "{env:" + providerKeyEnv + "}"
		}
	}
	cfg := map[string]any{"$schema": "https://opencode.ai/config.json", "provider": map[string]any{providerID: provider}}
	data, err := json.Marshal(cfg)
	if err != nil {
		return "", errors.New("OpenCode provider configuration could not be encoded")
	}
	dir := filepath.Join(home, "config", "opencode")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	if err = os.WriteFile(filepath.Join(dir, "opencode.json"), data, 0600); err != nil {
		return "", err
	}
	return key, nil
}

func selectedProviderKey(command *exec.Cmd) string {
	for _, value := range command.Env {
		if strings.HasPrefix(value, providerKeyEnv+"=") {
			return strings.TrimPrefix(value, providerKeyEnv+"=")
		}
	}
	return ""
}
