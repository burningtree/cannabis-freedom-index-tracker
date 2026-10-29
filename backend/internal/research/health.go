package research

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type HealthSnapshot struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	Version   string `json:"version,omitempty"`
	CheckedAt int64  `json:"checked_at"`
}
type Health struct {
	mu       sync.RWMutex
	snapshot HealthSnapshot
}

func NewHealth() *Health {
	return &Health{snapshot: HealthSnapshot{Status: "checking", Message: "Checking the OpenCode binary and a short model test prompt."}}
}
func (h *Health) Snapshot() HealthSnapshot { h.mu.RLock(); defer h.mu.RUnlock(); return h.snapshot }
func (h *Health) Ready() bool              { return h != nil && h.Snapshot().Status == "ready" }
func (h *Health) Set(s HealthSnapshot)     { h.mu.Lock(); h.snapshot = s; h.mu.Unlock() }

// Probe invokes the same actual OpenCode harness/model/auth path used by research,
// with a neutral workspace and a one-step no-tool agent instead of a real report.
func (r Runner) Probe(ctx context.Context) (string, error) {
	if r.Config.ResearchRunner == "local" {
		name := r.Config.OpenCodeBinary
		if name == "" {
			name = "opencode"
		}
		if _, err := exec.LookPath(name); err != nil {
			return "", errors.New("OpenCode binary is not accessible; install it or set OPENCODE_BIN")
		}
	}
	root, err := filepath.Abs(r.Config.Workspaces)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(root, ".startup-check-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	input := filepath.Join(dir, "input")
	if err = os.Mkdir(input, 0755); err != nil {
		return "", err
	}
	if err = os.WriteFile(filepath.Join(input, "opencode.json"), r.modelConfig(), 0644); err != nil {
		return "", err
	}
	args := []string{"run", "--pure", "--agent", "enforcement-probe", "--model", r.Config.ResearchModel, "--format", "json", "Reply exactly OK."}
	command, cleanup, err := r.commandArgs(ctx, input, "startup-check", args)
	if err != nil {
		return "", err
	}
	defer cleanup()
	version := "container"
	if r.Config.ResearchRunner == "local" {
		probe := exec.CommandContext(ctx, command.Path, append(append([]string(nil), command.Args[1:4]...), "--version")...)
		probe.Env = command.Env
		probe.Dir = command.Dir
		b, err := probe.Output()
		if err != nil {
			return "", errors.New("OpenCode binary could not run inside its filesystem sandbox")
		}
		version = strings.TrimSpace(string(b))

	}
	out := &capped{Max: 1024 * 1024}
	command.Stdout = out
	command.Stderr = &capped{Max: 256 * 1024}
	if err = command.Run(); err != nil {
		return version, probeFailure(out.Bytes())
	}
	text, err := FinalText(out.Bytes())
	if err != nil {
		return version, probeFailure(out.Bytes())
	}
	if !probeReplyOK(text) {
		return version, errors.New("OpenCode test prompt did not return the expected OK response")
	}
	return version, nil
}

// Some provider adapters include a preamble in their text event. Require the
// final response line to be the exact connectivity acknowledgement.
func probeReplyOK(text []byte) bool {
	lines := strings.Split(strings.TrimSpace(string(text)), "\n")
	return len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "OK"
}
func probeFailure(data []byte) error {
	// Never expose provider response bodies or credentials from the execution stream.
	for _, line := range strings.Split(string(data), "\n") {
		var event struct {
			Type  string `json:"type"`
			Error struct {
				Name string `json:"name"`
				Data struct {
					Status int `json:"statusCode"`
				} `json:"data"`
			} `json:"error"`
		}
		if json.Unmarshal([]byte(line), &event) == nil && event.Type == "error" {
			if event.Error.Data.Status == 401 || event.Error.Data.Status == 403 || strings.Contains(strings.ToLower(event.Error.Name), "auth") {
				return errors.New("OpenCode could not authenticate the selected provider; connect it in OpenCode and restart the backend")
			}
		}
	}
	return errors.New("OpenCode model test failed; verify opencode run --model opencode-go/deepseek-v4.1-flash in your terminal, then restart the backend")
}
func (r Runner) Check(ctx context.Context, h *Health) {
	check, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	version, err := r.Probe(check)
	state := HealthSnapshot{Status: "ready", Message: "OpenCode binary and model test passed. Queued requests are processed automatically.", Version: version, CheckedAt: time.Now().Unix()}
	if err != nil {
		state.Status = "failed"
		state.Message = err.Error()
	}
	h.Set(state)
	if r.Store != nil {
		_, _ = r.Store.DB.ExecContext(ctx, "UPDATE submission_feed_version SET version=version+1 WHERE singleton=1")
	}
}
