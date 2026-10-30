package research

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReportRecordsServiceSuppliedCreatorModel(t *testing.T) {
	r := valid()
	r.CreatedByModel = "opencode-go/deepseek-v4.1-flash#max"
	var header map[string]any
	if err := yaml.Unmarshal([]byte(strings.SplitN(r.Markdown(), "---", 3)[1]), &header); err != nil {
		t.Fatal(err)
	}
	if header["created_by_model"] != r.CreatedByModel {
		t.Fatal("lost exact provider/model/variant", header)
	}
	if _, err := Strict([]byte(`{"created_by_model":"invented/model"}`)); err == nil {
		t.Fatal("model output can spoof service attribution")
	}
}
func TestUpdatesPreserveCreatorAndRemoveLegacyUpdateModels(t *testing.T) {
	for _, creator := range []string{"original/provider-model", ""} {
		t.Run(creator, func(t *testing.T) {
			dir := t.TempDir()
			r := valid()
			r.Outcome = "existing_case"
			r.ExistingReport = r.Slug
			r.CreatedByModel = "opencode-go/deepseek-v4.1-flash#max"
			path := filepath.Join(dir, "reports/2026/01", r.Slug+".md")
			os.MkdirAll(filepath.Dir(path), 0755)
			header := "---\ndate: 2026-01-01\ncountry: th\ntype: raid\nstatus: reported\nupdated_by_models: [earlier/model]\nsources:\n  - title: News\n    url: https://news.example/a\n    publisher: Publisher\n"
			if creator != "" {
				header += "created_by_model: " + creator + "\n"
			}
			os.WriteFile(path, []byte(header+"---\nOriginal report. [@1]\n"), 0644)
			for i := 0; i < 2; i++ {
				d, err := PrepareDraft(dir, r)
				if err != nil {
					t.Fatal(err)
				}
				var metadata struct {
					Creator string   `yaml:"created_by_model"`
					Updated []string `yaml:"updated_by_models"`
				}
				if err = yaml.Unmarshal([]byte(strings.SplitN(d.Content, "---", 3)[1]), &metadata); err != nil {
					t.Fatal(err)
				}
				expectedCreator := creator
				if expectedCreator == "" {
					expectedCreator = "unknown"
				}
				if metadata.Creator != expectedCreator || len(metadata.Updated) != 0 {
					t.Fatal("creator replaced or retired update metadata retained", metadata)
				}
				if err = os.WriteFile(path, []byte(d.Content), 0644); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestPublicDecisionTitleIsBoundedAndOptional(t *testing.T) {
	r := valid()
	r.DecisionTitle = "Evidence does not identify the incident"
	if err := r.Validate(map[string]bool{"th": true}); err != nil {
		t.Fatal(err)
	}
	for _, title := range []string{strings.Repeat("x", 121), "<script>bad</script>"} {
		r.DecisionTitle = title
		if err := r.Validate(map[string]bool{"th": true}); err == nil {
			t.Fatal("unsafe or oversized decision title accepted")
		}
	}
	r.DecisionTitle = ""
	if err := r.Validate(map[string]bool{"th": true}); err != nil {
		t.Fatal("legacy result rejected", err)
	}
}
