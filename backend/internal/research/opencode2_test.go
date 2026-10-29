package research

import (
	"context"
	"encoding/json"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/config"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestV2FlagsUsePrivateServerWithoutLegacyPureFlag(t *testing.T) {
	args := strings.Join(v2Args([]string{"run", "--pure", "--model", "opencode-go/deepseek-v4.1-flash", "Reply OK"}), " ")
	if strings.Contains(args, "--pure") || !strings.Contains(args, "--standalone") {
		t.Fatal(args)
	}
	for _, version := range []string{"opencode v2.2.0", "2.2.0", "opencode v2.3.1"} {
		if !openCode2(version) {
			t.Fatal(version)
		}
	}
}
func TestStartupProbeUsesNativeV2CredentialsAndShortPrompt(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("local macOS adapter")
	}
	root := t.TempDir()
	binary := filepath.Join(root, "fake-opencode")
	script := `#!/bin/sh
case "$1" in
 --version) printf 'opencode v2.2.0\n';;
 auth) if [ "$2" = export ]; then printf '[{"integration":{"id":"console"},"credential":{"value":"fixture-secret"}}]\n'; else cat >/dev/null; fi;;
 run) printf '{"type":"text","part":{"text":"OK"}}\n';;
 *) exit 1;;
esac
`
	if err := os.WriteFile(binary, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r := Runner{Config: config.Config{ResearchRunner: "local", OpenCodeBinary: binary, ResearchModel: "opencode-go/deepseek-v4.1-flash", Workspaces: filepath.Join(root, "jobs")}}
	version, err := r.Probe(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != "opencode v2.2.0" {
		t.Fatal(version)
	}
}

func TestProbeChecksConnectivityRatherThanExactFormatting(t *testing.T) {
	for _, value := range []string{"OK", "\nOK\n", "OK\n\nMaximum steps for this agent have been reached. Summary: none.", "The requested response is OK. I will provide that.OK"} {
		if !probeReplyUsable([]byte(value)) {
			t.Fatal(value)
		}
	}
	for _, value := range []string{"", " \n \t"} {
		if probeReplyUsable([]byte(value)) {
			t.Fatal(value)
		}
	}
}
func TestV2ConfigurationDeniesToolsExceptResearchReads(t *testing.T) {
	input := t.TempDir()
	path := filepath.Join(input, ".opencode", "agents", "enforcement-researcher.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("---\npermission: legacy\n---\nResearch sources."), 0644); err != nil {
		t.Fatal(err)
	}
	if err := configureOpenCode2(input, 24); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(input, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["agent"] != nil || cfg["autoupdate"] != nil {
		t.Fatal("legacy config")
	}
	agents := cfg["agents"].(map[string]any)
	probe := agents["enforcement-probe"].(map[string]any)
	if probe["steps"] != float64(2) {
		t.Fatal("probe must not trigger the one-step summary", probe)
	}
	if len(probe["permissions"].([]any)) != 1 {
		t.Fatal("probe allows tools")
	}
	researcher := agents["enforcement-researcher"].(map[string]any)
	if researcher["system"] != "Research sources." {
		t.Fatal("lost agent instructions")
	}
	for _, item := range researcher["permissions"].([]any) {
		rule := item.(map[string]any)
		if rule["effect"] == "allow" && (rule["action"] == "shell" || rule["action"] == "edit" || rule["action"] == "subagent") {
			t.Fatal("unsafe permission")
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("legacy frontmatter still loaded")
	}
}
