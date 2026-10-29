package safeurl

import (
	"strings"
	"testing"
)

func page(title, body string) Evidence {
	return Evidence{URL: "https://www.example.com/a", FinalURL: "https://www.example.com/a", Content: "<html><head><title>" + title + "</title><script>var weed = 1;</script></head><body>" + body + "</body></html>"}
}

func TestCheckFindsTheWordForCannabis(t *testing.T) {
	c, err := Check(page("ตำรวจทำลายไร่กัญชา", "<p>"+strings.Repeat("เจ้าหน้าที่ตำรวจตระเวนชายแดนเข้าตรวจสอบพื้นที่ป่าสงวนแห่งชาติ ", 10)+"</p>"))
	if err != nil || c.Mention != "กัญชา" || c.Host != "example.com" || c.Title != "ตำรวจทำลายไร่กัญชา" {
		t.Fatalf("%+v %v", c, err)
	}
	if !strings.Contains(c.Describe(), "mentioning cannabis (กัญชา)") {
		t.Fatal(c.Describe())
	}
}

func TestCheckIgnoresScriptsAndPartWords(t *testing.T) {
	// "weed" only in a script and "hashtag" in the text are not mentions
	c, err := Check(page("Football results", "<p>"+strings.Repeat("The match ended two all. Share it with the hashtag #cup. ", 10)+"</p>"))
	if err != nil || c.Mention != "" {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestCheckRejectsAPageWithoutText(t *testing.T) {
	if _, err := Check(page("App", `<div id="root"></div>`)); err == nil || !strings.Contains(err.Error(), "almost no readable text") {
		t.Fatalf("want a no-text error, got %v", err)
	}
}

func TestTextWrapAndContains(t *testing.T) {
	page := `<html><script>var x</script><p>First   paragraph &amp; more.</p><div>Second<br>line</div></html>`
	if got := Text(page); got != "First paragraph & more.\nSecond\nline" {
		t.Fatalf("%q", got)
	}
	long := strings.Repeat("word ", 500)
	for _, line := range strings.Split(Wrap(long, 1000), "\n") {
		if len([]rune(line)) > 1000 {
			t.Fatal("line not wrapped")
		}
	}
	if !Contains(page, "paragraph & more. Second") || Contains(page, "invented quote") {
		t.Fatal("quotation check")
	}
}
