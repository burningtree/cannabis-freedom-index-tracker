package api

import (
	"encoding/json"
	"testing"
)

func TestStatusCountsWaitingRequests(t *testing.T) {
	_, h := server(t)
	if w := request(h, "POST", "/v1/submissions", `{"url":"https://news.example/raid"}`, "https://tracker.example"); w.Code != 202 {
		t.Fatal(w.Code, w.Body.String())
	}
	w := request(h, "GET", "/v1/status", "", "")
	var v struct {
		Online  bool  `json:"online"`
		Running int64 `json:"running"`
		Queued  int64 `json:"queued"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &v) != nil || !v.Online || v.Queued != 1 || v.Running != 0 {
		t.Fatal(w.Code, w.Body.String())
	}
}
