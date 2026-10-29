package api

import (
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/research"
	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/store"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
)

type ProcessingInfo struct {
	Startup               *research.HealthSnapshot `json:"startup,omitempty"`
	RawLogsAvailable      bool                     `json:"raw_logs_available"`
	Mode                  string                   `json:"mode"`
	Model                 string                   `json:"model"`
	Runner                string                   `json:"runner"`
	Enabled               bool                     `json:"enabled"`
	Reason                string                   `json:"reason"`
	AutomaticPublication  bool                     `json:"automatic_publication"`
	PublicationConfigured bool                     `json:"publication_configured"`
}

func (s *Server) processingInfo() ProcessingInfo {
	c := s.Config
	p := ProcessingInfo{RawLogsAvailable: c.RawAILogs, Mode: "automatic", Model: c.ResearchModel, Runner: "OpenCode (" + c.ResearchRunner + ")", Enabled: c.ResearchEnabled(), AutomaticPublication: c.AutoPublish, PublicationConfigured: c.GitHubAppID != ""}
	if !c.AutoPublish {
		p.Mode = "review"
	}
	if c.Demo {
		p.Mode = "demo"
		p.Enabled = true
		p.Reason = "Demo worker simulates progress; no AI model is called."
		return p
	}
	if c.ResearchModel == "" {
		p.Reason = "AI processing is not configured. Set RESEARCH_MODEL and configure OpenCode auth."
		p.Enabled = false
	} else if c.ResearchRunner != "local" && c.ResearchKey == "" {
		p.Reason = "Waiting for the AI API key. Add RESEARCH_API_KEY to .env and restart backend-dev."
		p.Enabled = false
	} else if c.ResearchRunner == "local" {
		binary := c.OpenCodeBinary
		if binary == "" {
			binary = "opencode"
		}
		if _, err := exec.LookPath(binary); err != nil {
			p.Enabled = false
			p.Reason = "The OpenCode CLI is unavailable."
		} else if runtime.GOOS != "darwin" {
			p.Enabled = false
			p.Reason = "The local filesystem sandbox requires macOS. Configure the Docker runner on this system."
		}
	} else if c.ResearchImage == "" {
		p.Enabled = false
		p.Reason = "The OpenCode Docker image and network are not configured."
	}
	if p.Enabled && p.Reason == "" {
		p.Reason = "OpenCode processes queued requests automatically."
	}
	if s.Health != nil && !c.Demo {
		snapshot := s.Health.Snapshot()
		p.Startup = &snapshot
		p.Enabled = p.Enabled && snapshot.Status == "ready"
		p.Reason = snapshot.Message
	}
	return p
}
func (s *Server) ai(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		problem(w, 404, "Receipt not found.")
		return
	}
	after := int64(-1)
	if raw := r.URL.Query().Get("after"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 0 {
			problem(w, 400, "Invalid AI activity cursor.")
			return
		}
		after = value
	}
	activity, err := s.Store.AIActivity(r.Context(), id, after)
	if err != nil {
		if store.IsMissingAI(err) {
			problem(w, 404, "Receipt not found.")
		} else {
			problem(w, 503, "AI activity unavailable.")
		}
		return
	}
	write(w, 200, struct {
		Processing ProcessingInfo `json:"processing"`
		Activity   any            `json:"activity"`
	}{s.processingInfo(), activity})
}

func (s *Server) aiLog(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		problem(w, 404, "Receipt not found.")
		return
	}
	activity, err := s.Store.AIActivityLog(r.Context(), id)
	if err != nil {
		if store.IsMissingAI(err) {
			problem(w, 404, "Receipt not found.")
		} else {
			problem(w, 503, "AI log unavailable.")
		}
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	write(w, 200, struct {
		SchemaVersion int              `json:"schema_version"`
		SubmissionID  string           `json:"submission_id"`
		Scope         string           `json:"scope"`
		Processing    ProcessingInfo   `json:"processing"`
		Activity      store.AIActivity `json:"activity"`
	}{1, id, "All recorded public AI activity; raw tool output and private reasoning are excluded.", s.processingInfo(), activity})
}
