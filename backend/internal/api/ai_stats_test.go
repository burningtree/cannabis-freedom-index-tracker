package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAIStatsPeriodsAndPublicProjection(t *testing.T) {
	s, _ := server(t)
	for _, period := range []string{"all", "30d", "today"} {
		response := request(s.Handler(), "GET", "/v1/stats/ai?period="+period, "", "")
		var payload struct {
			Period string `json:"period"`
			Since  int64  `json:"since"`
			Usage  []any  `json:"usage"`
		}
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &payload) != nil || payload.Period != period || payload.Usage == nil {
			t.Fatal(response.Code, response.Body.String())
		}
		if period != "all" && payload.Since <= 0 {
			t.Fatal(payload)
		}
		for _, private := range []string{"workspace", "session_id", "case_id", "prompt"} {
			if strings.Contains(response.Body.String(), private) {
				t.Fatal("private field exposed", response.Body.String())
			}
		}
	}
	if response := request(s.Handler(), "GET", "/v1/stats/ai?period=invalid", "", ""); response.Code != 400 {
		t.Fatal(response.Code)
	}
}
