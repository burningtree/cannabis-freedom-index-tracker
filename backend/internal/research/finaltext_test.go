package research

import (
	"strings"
	"testing"
)

func textEvent(s string) string {
	return `{"type":"text","part":{"text":` + strings.ReplaceAll(strings.ReplaceAll(`"`+strings.ReplaceAll(s, `"`, `\"`)+`"`, "\n", `\n`), "\t", `\t`) + `}}` + "\n"
}

// An answer written after a few words of prose is still found.
func TestFinalTextFindsTheAnswerAfterProse(t *testing.T) {
	got, err := FinalText([]byte(textEvent("I have enough now. Here it is: {\"outcome\":\"not_relevant\",\"note\":\"{braces} inside\"} done")))
	if err != nil || string(got) != `{"outcome":"not_relevant","note":"{braces} inside"}` {
		t.Fatalf("got %s, %v", got, err)
	}
	// notes with no JSON come back as text; the research run then reports that it got no answer
	if got, err = FinalText([]byte(textEvent("Let me finalize facts with exact quotes."))); err != nil || string(got) != "Let me finalize facts with exact quotes." {
		t.Fatalf("got %s, %v", got, err)
	}
}
