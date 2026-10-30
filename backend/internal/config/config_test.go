package config

import (
	"strings"
	"testing"
)

func defaults(t *testing.T) {
	t.Helper()
	for _, k := range []string{"NOSTR_GATEWAY_SECRET", "NOSTR_BOT_SECRET", "NOSTR_RELAYS", "RESEARCH_IMAGE", "RESEARCH_NETWORK", "RESEARCH_MODEL", "RESEARCH_API_KEY", "GITHUB_APP_ID", "GITHUB_INSTALLATION_ID", "GITHUB_APP_PRIVATE_KEY_FILE", "INGEST_TOKEN", "TRUSTED_PROXY_CIDRS", "HARVEST_RSS_URLS", "HARVEST_GDELT_QUERIES"} {
		t.Setenv(k, "")
	}
	t.Setenv("DEMO_WORKER", "false")
}
func TestOptionalIntegrationsDisabledByDefault(t *testing.T) {
	defaults(t)
	c, err := Load()
	if err != nil || c.ResearchImage != "" || c.BotSecret != "" {
		t.Fatal(err, c)
	}
}
func TestInvalidConfigurationFailsClosed(t *testing.T) {
	for _, value := range []struct{ key, value string }{{"CORS_ORIGINS", "*"}, {"NOSTR_BOT_SECRET", strings.Repeat("a", 64)}, {"RESEARCH_IMAGE", "image:latest"}, {"GITHUB_REPO", "invalid"}, {"TRUSTED_PROXY_CIDRS", "anyone"}, {"INGEST_TOKEN", "short"}} {
		t.Run(value.key, func(t *testing.T) {
			defaults(t)
			t.Setenv(value.key, value.value)
			if _, err := Load(); err == nil {
				t.Fatal("accepted invalid config")
			}
		})
	}
}

func TestDailyBudgetZeroIsUnlimitedButOtherBoundsRemain(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{{"0", true}, {"100", true}, {"99", false}, {"-1", false}, {"invalid", false}} {
		t.Run(test.value, func(t *testing.T) {
			defaults(t)
			t.Setenv("RESEARCH_RUN_CENTS", "100")
			t.Setenv("RESEARCH_DAILY_CENTS", test.value)
			_, err := Load()
			if (err == nil) != test.valid {
				t.Fatal(test, err)
			}
		})
	}
}

func TestResearchModelComesFromEnvironment(t *testing.T) {
	for _, runner := range []string{"local", "docker"} {
		t.Run(runner, func(t *testing.T) {
			defaults(t)
			t.Setenv("RESEARCH_RUNNER", runner)
			c, err := Load()
			if err != nil || c.ResearchModel != "" || c.ResearchEnabled() {
				t.Fatalf("missing model should disable research: model=%q err=%v", c.ResearchModel, err)
			}
			t.Setenv("RESEARCH_MODEL", "other-provider/selected-model#high")
			c, err = Load()
			if err != nil || c.ResearchModel != "other-provider/selected-model#high" {
				t.Fatalf("configured model was not preserved: model=%q err=%v", c.ResearchModel, err)
			}
			if runner == "local" && !c.ResearchEnabled() {
				t.Fatal("configured local model should enable research")
			}
		})
	}
}
