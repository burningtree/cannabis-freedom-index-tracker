package research

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
	"math"
	"regexp"
	"strings"
)

type executionEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionID"`
	Error     struct {
		Name string `json:"name"`
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	} `json:"error"`
	Part struct {
		SessionID string `json:"sessionID"`
		Text      string `json:"text"`
		Tool      string `json:"tool"`
		State     struct {
			Status string          `json:"status"`
			Error  string          `json:"error"`
			Input  json.RawMessage `json:"input"`
			Title  string          `json:"title"`
		} `json:"state"`
		Cost   float64 `json:"cost"`
		Tokens struct {
			Input     int64 `json:"input"`
			Output    int64 `json:"output"`
			Reasoning int64 `json:"reasoning"` // billed as output
			Cache     struct {
				Read  int64 `json:"read"`
				Write int64 `json:"write"`
			} `json:"cache"`
		} `json:"tokens"`
	} `json:"part"`
}
type eventWriter struct {
	ctx              context.Context
	store            *store.Store
	cid, run, secret string
	buffer           []byte
	events           int
	steps            int // model steps finished, for the step dividers in the public chat
	failed           int // chat messages that could not be stored
}

func (w *eventWriter) Write(p []byte) (int, error) {
	w.buffer = append(w.buffer, p...)
	if len(w.buffer) > 4*1024*1024 {
		return 0, fmt.Errorf("event size exceeded")
	}
	for {
		index := bytes.IndexByte(w.buffer, '\n')
		if index < 0 {
			break
		}
		line := append([]byte(nil), w.buffer[:index]...)
		w.buffer = w.buffer[index+1:]
		if len(bytes.TrimSpace(line)) > 0 {
			if err := w.handle(line); err != nil {
				return 0, err
			}
		}
	}
	return len(p), nil
}

var sessionPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,120}$`)
var secretPattern = regexp.MustCompile(`(?i)(bearer\s+\S+|(?:sk|oc|opencode)[-_][A-Za-z0-9_-]{16,})`)

// what a tool did, in words for the public chat, followed by what it was asked (all public data)
var toolNames = map[string]string{
	"read": "Reading research context", "glob": "Locating research files", "grep": "Checking existing report coverage",
	"list": "Listing research files", "skill": "Loading editorial instructions", "webfetch": "Opening a public source",
	"websearch": "Searching for corroborating coverage", "write": "Writing the draft report", "edit": "Revising the draft report",
	"patch": "Revising the draft report", "todowrite": "Planning the next steps", "todoread": "Reviewing the plan",
	"task": "Asking a helper agent", "bash": "Running a check",
}
var toolPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,30}$`)

// a tool error, on one line, credentials redacted
func (w *eventWriter) reason(text string) string {
	return w.safeN(strings.Join(strings.Fields(text), " "), 500)
}

func thousands(n int64) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func (w *eventWriter) safe(text string) string { return w.safeN(text, 1000) }

// the text with credentials redacted, cut to max characters; everything else is public data
func (w *eventWriter) safeN(text string, max int) string {
	if w.secret != "" {
		text = strings.ReplaceAll(text, w.secret, "[redacted]")
	}
	text = secretPattern.ReplaceAllString(text, "[redacted]")
	r := []rune(text)
	if len(r) > max {
		text = string(r[:max]) + "…"
	}
	return text
}

// what a tool was asked to do: the address, the query, the file or the pattern
func (w *eventWriter) toolInput(raw json.RawMessage) string {
	var in map[string]any
	if json.Unmarshal(raw, &in) != nil {
		return ""
	}
	for _, key := range []string{"url", "query", "filePath", "file_path", "path", "pattern", "name", "description"} {
		if v, ok := in[key].(string); ok && strings.TrimSpace(v) != "" {
			if key == "pattern" {
				if inc, ok := in["include"].(string); ok && inc != "" {
					v += " in " + inc
				}
			}
			return w.safeN(strings.Join(strings.Fields(v), " "), 300)
		}
	}
	return ""
}
func (w *eventWriter) handle(line []byte) error {
	var e executionEvent
	if json.Unmarshal(line, &e) != nil {
		return nil
	}
	session := e.SessionID
	if session == "" {
		session = e.Part.SessionID
	}
	if sessionPattern.MatchString(session) {
		if _, err := w.store.DB.ExecContext(w.ctx, "UPDATE research_runs SET session_id=? WHERE id=? AND session_id<>?", session, w.run, session); err != nil {
			return err
		}
	}
	stepDone := ""
	if e.Type == "step_finish" {
		if e.Part.Tokens.Input >= 0 && e.Part.Tokens.Output >= 0 && e.Part.Tokens.Input < 20000000 && e.Part.Tokens.Output < 20000000 && !math.IsNaN(e.Part.Cost) && e.Part.Cost >= 0 && e.Part.Cost < 1000 {
			t := e.Part.Tokens
			if _, err := w.store.DB.ExecContext(w.ctx, "UPDATE research_runs SET input_tokens=input_tokens+?,output_tokens=output_tokens+?,reasoning_tokens=reasoning_tokens+?,cache_read_tokens=cache_read_tokens+?,cache_write_tokens=cache_write_tokens+?,cost_usd=cost_usd+?,tokens_backfilled=1 WHERE id=?", t.Input, t.Output, max(0, t.Reasoning), max(0, t.Cache.Read), max(0, t.Cache.Write), e.Part.Cost, w.run); err != nil {
				return err
			}
			w.steps++
			stepDone = fmt.Sprintf("Step %d done · %s tokens read, %s written", w.steps, thousands(e.Part.Tokens.Input+e.Part.Tokens.Cache.Read), thousands(e.Part.Tokens.Output+e.Part.Tokens.Reasoning))
			if e.Part.Tokens.Reasoning > 0 {
				stepDone += fmt.Sprintf(" (%s of them reasoning)", thousands(e.Part.Tokens.Reasoning))
			}
		}
	}
	if w.events >= 500 { // a long run keeps its last steps out of the chat rather than flooding it
		return nil
	}
	role, kind, text := "", "", ""
	switch e.Type {
	case "step_finish":
		// a step's start says nothing; its end says how much the model read and wrote
		role, kind, text = "system", "step", stepDone
	case "tool_use":
		name, ok := toolNames[e.Part.Tool]
		if !ok && toolPattern.MatchString(e.Part.Tool) {
			name, ok = "Using the "+e.Part.Tool+" tool", true
		}
		if ok {
			role, kind, text = "tool", "tool", name
			if what := w.toolInput(e.Part.State.Input); what != "" {
				text += ": " + what
			}
			switch e.Part.State.Status {
			case "completed":
				text += " — done."
			case "error":
				text += " — failed"
				if why := w.reason(e.Part.State.Error); why != "" {
					text += ": " + why
				}
				text += "."
			default:
				text += "…"
			}
		}
	case "text":
		value := strings.TrimSpace(e.Part.Text)
		switch {
		case value == "":
		case strings.HasPrefix(value, "{") || strings.HasPrefix(value, "```"):
			// the structured final answer, shown folded in the chat
			role, kind, text = "assistant", "answer", w.safeN(value, 20000)
		default:
			role, kind, text = "assistant", "message", w.safe(value)
		}
	case "reasoning":
		if value := strings.TrimSpace(e.Part.Text); value != "" {
			role, kind, text = "assistant", "reasoning", w.safeN(value, 4000)
		}
	case "error":
		why := strings.TrimSpace(e.Error.Data.Message)
		if why == "" {
			why = e.Error.Name
		}
		text = "OpenCode reported an error"
		if why != "" {
			text += ": " + w.reason(why)
		}
		role, kind, text = "system", "error", text+"."
	}
	if text == "" {
		return nil
	}
	w.events++
	// the chat is a view of the run: a message that cannot be stored must not stop OpenCode
	if err := w.store.AddAIEvent(w.ctx, w.cid, w.run, role, kind, text); err != nil {
		w.failed++
	}
	return nil
}
