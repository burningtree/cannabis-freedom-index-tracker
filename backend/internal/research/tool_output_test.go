package research

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func toolOutputAgent(t *testing.T, input string) {
	t.Helper()
	p := filepath.Join(input, ".opencode/agents/enforcement-researcher.md")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("---\ndescription: Research sources\nmode: primary\npermission:\n  '*': deny\n  read: allow\n  grep: allow\n  external_directory: deny\n  bash: deny\n  edit: deny\n---\nResearch only.\n"), 0644); err != nil {
		t.Fatal(err)
	}
}
func TestToolOutputExternalAccessIsScopedInBothHarnessVersions(t *testing.T) {
	for _, v2 := range []bool{false, true} {
		name := "legacy"
		if v2 {
			name = "v2"
		}
		t.Run(name, func(t *testing.T) {
			input := t.TempDir()
			toolOutputAgent(t, input)
			output := filepath.Join(t.TempDir(), "data/opencode/tool-output")
			if v2 {
				if err := configureOpenCode2(input, 24, output); err != nil {
					t.Fatal(err)
				}
				b, _ := os.ReadFile(filepath.Join(input, "opencode.json"))
				var cfg struct {
					Permissions []map[string]string
					Agents      map[string]struct{ Permissions []map[string]string }
				}
				if err := json.Unmarshal(b, &cfg); err != nil {
					t.Fatal(err)
				}
				allows := 0
				for _, rule := range cfg.Agents["enforcement-researcher"].Permissions {
					if rule["action"] != "external_directory" || rule["effect"] != "allow" {
						continue
					}
					allows++
					if rule["resource"] != filepath.ToSlash(output)+"/*" {
						t.Fatal("broad external access", rule)
					}
				}
				if allows != 1 || len(cfg.Agents["enforcement-probe"].Permissions) != 1 || len(cfg.Permissions) != 1 {
					t.Fatal(cfg)
				}
			} else {
				if err := configureLegacyToolOutput(input, output); err != nil {
					t.Fatal(err)
				}
				b, _ := os.ReadFile(filepath.Join(input, ".opencode/agents/enforcement-researcher.md"))
				parts := strings.SplitN(string(b), "---", 3)
				var cfg struct{ Permission map[string]any }
				if err := yaml.Unmarshal([]byte(parts[1]), &cfg); err != nil {
					t.Fatal(err)
				}
				external := cfg.Permission["external_directory"].(map[string]any)
				if len(external) != 2 || external["*"] != "deny" || external[filepath.ToSlash(output)+"/*"] != "allow" || cfg.Permission["bash"] != "deny" || cfg.Permission["edit"] != "deny" || !strings.Contains(parts[2], "Research only.") {
					t.Fatal(cfg)
				}
			}
		})
	}
}

// Read the installed harness's effective rules, without provider calls or saved auth.
func TestNativeV2LoadsScopedToolOutputPermission(t *testing.T) {
	binary, err := exec.LookPath("opencode")
	if override := os.Getenv("TEST_OPENCODE_V2_BIN"); override != "" {
		binary = override
		err = nil
	}
	if err != nil {
		t.Skip("OpenCode unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	version, err := localVersion(ctx, binary)
	if err != nil || !openCode2(version) {
		t.Skip("native OpenCode 2 integration test")
	}
	input := t.TempDir()
	toolOutputAgent(t, input)
	home := t.TempDir()
	output := filepath.Join(home, "data/opencode/tool-output")
	if err = configureOpenCode2(input, 24, output); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, binary, "debug", "agents", "--standalone")
	cmd.Dir = input
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "XDG_CONFIG_HOME=" + filepath.Join(home, "config"), "XDG_DATA_HOME=" + filepath.Join(home, "data"), "XDG_CACHE_HOME=" + filepath.Join(home, "cache"), "XDG_STATE_HOME=" + filepath.Join(home, "state"), "CI=true"}
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("native config: %v %s", err, b)
	}
	var agents []struct {
		Name        string
		Permission  []map[string]string
		Permissions []map[string]string
	}
	if err = json.Unmarshal(b, &agents); err != nil {
		t.Fatalf("invalid debug agents: %v %.300s", err, b)
	}
	found := false
	for _, agent := range agents {
		if agent.Name != "enforcement-researcher" {
			continue
		}
		for _, rule := range append(agent.Permission, agent.Permissions...) {
			if rule["action"] == "external_directory" && rule["effect"] == "allow" && rule["resource"] == filepath.ToSlash(output)+"/*" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("native harness did not retain scoped output permission: %.1500s", b)
	}
}
