package research

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/config"
)

func providerFixture(t *testing.T, path, key string) {
	t.Helper()
	data, err := json.Marshal(map[string]any{
		"plugin":     []string{"never-copy-this-plugin"},
		"permission": map[string]string{"*": "allow"},
		"provider": map[string]any{
			"custom":    map[string]any{"npm": "@ai-sdk/openai-compatible", "name": "Custom provider", "options": map[string]string{"baseURL": "https://example.org/v1", "apiKey": key}, "models": map[string]any{"custom-model": map[string]any{"name": "Custom model", "limit": map[string]int{"context": 120000, "output": 8000}}}},
			"unrelated": map[string]any{"options": map[string]string{"apiKey": "unrelated-secret"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLocalProviderConfigIsScopedAndPrivate(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	source := filepath.Join(root, ".opencode", "opencode.json")
	providerFixture(t, source, "custom-private-key")
	r := Runner{Config: config.Config{ResearchRunner: "local", ResearchModel: "custom/custom-model#high"}}
	home := filepath.Join(root, "session")
	key, err := r.copyLocalProvider(home)
	if err != nil || key != "custom-private-key" {
		t.Fatalf("provider key not loaded: %v", err)
	}
	target := filepath.Join(home, "config", "opencode", "opencode.json")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"custom-private-key", "unrelated-secret", "never-copy-this-plugin", "permission"} {
		if strings.Contains(string(data), value) {
			t.Fatal("unrelated settings or credential written to session config")
		}
	}
	var cfg struct {
		Provider map[string]struct {
			Options map[string]string
			Models  map[string]any
		}
	}
	if err = json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Provider) != 1 || cfg.Provider["custom"].Options["baseURL"] != "https://example.org/v1" || cfg.Provider["custom"].Models["custom-model"] == nil {
		t.Fatal("custom endpoint/model configuration lost")
	}
	info, _ := os.Stat(target)
	if info.Mode().Perm() != 0600 {
		t.Fatal("provider configuration is not private")
	}
	// Explicit file overrides and provider-scoped environment references work too.
	explicit := filepath.Join(root, "explicit.json")
	providerFixture(t, explicit, "{env:CUSTOM_TEST_PROVIDER_KEY}")
	t.Setenv("CUSTOM_TEST_PROVIDER_KEY", "env-test-key")
	r.Config.OpenCodeConfigFile = explicit
	if key, err = r.copyLocalProvider(home); err != nil || key != "env-test-key" {
		t.Fatalf("explicit configuration did not resolve selected credential: %v", err)
	}
}

func TestLocalProviderConfigErrorsDoNotExposeContents(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "broken.json")
	os.WriteFile(source, []byte(`{"provider":{"custom":{"options":{"apiKey":"private-value"}}},broken`), 0600)
	r := Runner{Config: config.Config{ResearchModel: "custom/model", OpenCodeConfigFile: source}}
	_, err := r.copyLocalProvider(filepath.Join(root, "session"))
	if err == nil || strings.Contains(err.Error(), "private-value") {
		t.Fatal("invalid config not rejected privately")
	}
}

func TestNativeLocalHarnessLoadsCustomProviderFromHomeConfig(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS sandbox")
	}
	binary, err := exec.LookPath("opencode")
	if err != nil {
		t.Skip("OpenCode not installed")
	}
	version, err := localVersion(context.Background(), binary)
	if err != nil {
		t.Fatal(err)
	}
	if openCode2(version) {
		t.Skip("legacy debug config command; v2 tested with fake harness")
	}
	root := t.TempDir()
	source := filepath.Join(root, "provider.json")
	providerFixture(t, source, "custom-private-key")
	input := filepath.Join(root, "input")
	os.MkdirAll(input, 0700)
	r := Runner{Config: config.Config{ResearchRunner: "local", OpenCodeBinary: binary, ResearchModel: "custom/custom-model", OpenCodeConfigFile: source, OpenCodeAuthFile: filepath.Join(root, "missing-auth.json")}}
	os.WriteFile(filepath.Join(input, "opencode.json"), r.modelConfig(), 0600)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd, cleanup, err := r.command(ctx, input, "provider-config-test")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	cmd.Args = append(cmd.Args[:4], "debug", "config")
	data, err := cmd.Output()
	if err != nil {
		t.Fatal("native CLI could not load isolated configuration", err)
	}
	var cfg struct {
		Provider map[string]struct {
			Options map[string]string
			Models  map[string]any
		}
	}
	if err = json.Unmarshal(data, &cfg); err != nil {
		t.Fatal("native configuration was not JSON")
	}
	if cfg.Provider["custom"].Options["baseURL"] != "https://example.org/v1" || cfg.Provider["custom"].Options["apiKey"] != "custom-private-key" || cfg.Provider["custom"].Models["custom-model"] == nil {
		t.Fatal("native harness did not resolve custom provider endpoint, model and credentials")
	}
	if cfg.Provider["unrelated"].Options != nil {
		t.Fatal("unrelated provider escaped selection")
	}
	if selectedProviderKey(cmd) != "custom-private-key" {
		t.Fatal("selected key missing for diagnostics redaction")
	}
}
