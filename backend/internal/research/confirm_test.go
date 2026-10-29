package research

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/burningtree/cannabis-freedom-index-tracker/backend/internal/safeurl"
)

// A second source that blocks the service (403) does not sink a report the first source confirms;
// a fact whose quotation is missing is left out; most quotations missing is refused.
func TestConfirmKeepsSoundAnswers(t *testing.T) {
	r := valid()
	r.Sources = append(r.Sources, Source{URL: "https://blocked.example/b", Title: "Second story", Publisher: "Blocker"}, Source{URL: "https://other.example/c", Title: "Third story", Publisher: "Other"})
	r.Facts = []Fact{
		{Text: "Police raided the farm.", Source: 1, Quote: "Police raided the farm in the province."},
		{Text: "Two men were held.", Source: 2, Quote: "two men were held by the police"},
		{Text: "An invented sentence.", Source: 3, Quote: "this sentence is nowhere in the page"},
		{Text: "The farm was large.", Source: 1, Quote: "a large farm of ten hectares"},
	}
	initial := safeurl.Evidence{URL: r.Sources[0].URL, Content: "Police raided the farm in the province. It was a large farm of ten hectares."}
	fetch := func(_ context.Context, u string) (safeurl.Evidence, error) {
		if strings.Contains(u, "blocked") {
			return safeurl.Evidence{}, errors.New("the site answered HTTP 403")
		}
		return safeurl.Evidence{URL: u, Content: "Something else entirely, long enough to be a page."}, nil
	}
	got, evidence, check, err := r.Confirm(context.Background(), initial, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if !check.Passed || check.Checked != 3 || check.Confirmed != 2 || len(check.LeftOut) != 1 || check.Sources[1].Opened || check.Sources[1].Facts != 1 {
		t.Fatalf("%+v", check)
	}
	if len(got.Facts) != 3 || len(got.Sources) != 2 || len(evidence) != 2 {
		t.Fatalf("facts %d sources %d evidence %d", len(got.Facts), len(got.Sources), len(evidence))
	}
	for _, f := range got.Facts {
		if f.Source < 1 || f.Source > len(got.Sources) || f.Text == "An invented sentence." {
			t.Fatal(got.Facts)
		}
	}
	if err := got.Validate(map[string]bool{"th": true}); err != nil || len(got.Uncertainties) != 2 {
		t.Log(err)
		t.Fatal(got.Uncertainties)
	}
	// most quotations missing: refused
	r.Facts = []Fact{r.Facts[0], {Text: "Invented one.", Source: 3, Quote: "this one is simply not there"}, {Text: "Invented two.", Source: 3, Quote: "and this one is not there either"}}
	if _, _, check, err = r.Confirm(context.Background(), initial, fetch); err == nil || check.Passed || check.Reason == "" {
		t.Fatal("mostly invented answer accepted")
	}
}
