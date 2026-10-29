package research

import (
	"strings"
	"testing"
)

// The answer becomes a file laid out like the hand-written reports: readable YAML, paragraphs.
func TestMarkdownLooksLikeAReport(t *testing.T) {
	r := valid()
	arrested, kg := 2, 12.5
	r.Also = []string{"arrest"}
	r.Figures = &Figures{Arrested: &arrested, KgSeized: &kg, Value: &Money{Amount: 300000, Currency: "THB"}}
	r.Tags = []string{"opium"}
	r.Sources[0].Date = "2026-01-03"
	r.Sources[0].Language = "th"
	r.Summary = strings.Repeat("Police raided a farm and took two men away. ", 4)
	r.Facts = []Fact{
		{Text: "Police raided the farm.", Source: 1, Quote: "q", Paragraph: 1},
		{Text: "Two men were arrested.", Source: 1, Quote: "q", Paragraph: 1},
		{Text: "The farm grew opium too.", Source: 1, Quote: "q", Paragraph: 2},
	}
	md := r.Markdown()
	for _, want := range []string{
		"---\ndate: 2026-01-02\ncountry: th\nplace: Chiang Mai\ncoordinates: [18.79, 98.98]\nprecision: locality\ntype: raid\nalso: [arrest]\n",
		"summary: >-\n  Police raided",
		"figures:\n  arrested: 2\n  kg_seized: 12.5\n  value:\n    amount: 300000\n    currency: THB\n",
		"status: reported\ncreated_by_model: fixture/research-model\ntags: [opium]\nsources:\n  - title: News story\n    url: https://news.example/a\n    publisher: Publisher\n    date: 2026-01-03\n    language: th\n---\n",
		"\n\nPolice raided the farm. [@1] Two men were arrested. [@1]\n\nThe farm grew opium too. [@1]\n",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("missing %q in\n%s", want, md)
		}
	}
	// model text that looks like YAML stays a string
	r.Title = "status: verified\nx"
	if strings.Contains(r.Markdown(), "\nstatus: verified") {
		t.Fatal("title escaped into front matter")
	}
}

// Photos of the event are linked as media, only for pages the report cites.
func TestMediaLinksOnlyForCitedPages(t *testing.T) {
	r := valid()
	r.Media = []Media{{Kind: "photo", URL: r.Sources[0].URL, Publisher: "Publisher", Caption: "Police photos of the plants. Photo: Police"}}
	if err := r.Validate(map[string]bool{"th": true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Markdown(), "media:\n  - kind: photo\n    url: https://news.example/a\n    publisher: Publisher\n    caption: \"Police photos of the plants. Photo: Police\"\n") && !strings.Contains(r.Markdown(), "caption: 'Police photos of the plants. Photo: Police'") {
		t.Fatal(r.Markdown())
	}
	r.Media[0].URL = "https://elsewhere.example/photo"
	if r.Validate(map[string]bool{"th": true}) == nil {
		t.Fatal("media link to an uncited page accepted")
	}
}
