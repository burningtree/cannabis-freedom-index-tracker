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
