package safeurl

import (
	_ "embed"
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Generated from CFI /api/glossary.json by node scripts/sync-glossary.mjs.
// Keep a reviewed snapshot in the binary: startup/research never depends on a live API.
//
//go:embed cannabis-glossary.json
var glossaryJSON []byte

type recognitionRule struct {
	text, label        string
	prefix, continuous bool
}

var cannabisRules = loadCannabisRules()

func foldTerm(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return unicode.ToLower(r)
	}, norm.NFKD.String(text))
}
func continuousScript(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Thai, r) || unicode.Is(unicode.Lao, r) || unicode.Is(unicode.Khmer, r) || unicode.Is(unicode.Myanmar, r) {
			return true
		}
	}
	return false
}
func loadCannabisRules() []recognitionRule {
	var data struct {
		Version     int
		Recognition struct{ Words, Prefixes []string }
	}
	if err := json.Unmarshal(glossaryJSON, &data); err != nil || data.Version != 1 || len(data.Recognition.Words) == 0 {
		panic("invalid embedded CFI cannabis glossary")
	}
	rules := []recognitionRule{}
	seen := map[string]bool{}
	for _, word := range data.Recognition.Words {
		folded := foldTerm(word)
		if folded == "" || seen[folded] {
			continue
		}
		seen[folded] = true
		rules = append(rules, recognitionRule{text: folded, label: word, continuous: continuousScript(folded)})
	}
	for _, prefix := range data.Recognition.Prefixes {
		rules = append(rules, recognitionRule{text: foldTerm(prefix), label: prefix, prefix: true})
	}
	return rules
}
func wordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) }

func cannabisMention(text string) string {
	folded := foldTerm(text)
	for _, rule := range cannabisRules {
		for offset := 0; offset < len(folded); {
			at := strings.Index(folded[offset:], rule.text)
			if at < 0 {
				break
			}
			start := offset + at
			end := start + len(rule.text)
			offset = end
			if !rule.continuous && start > 0 {
				before, _ := utf8.DecodeLastRuneInString(folded[:start])
				if wordRune(before) {
					continue
				}
			}
			if !rule.prefix && !rule.continuous && end < len(folded) {
				after, _ := utf8.DecodeRuneInString(folded[end:])
				if wordRune(after) {
					continue
				}
			}
			if rule.prefix {
				for end < len(folded) {
					after, size := utf8.DecodeRuneInString(folded[end:])
					if !wordRune(after) {
						break
					}
					end += size
				}
				return folded[start:end]
			}
			return rule.label
		}
	}
	return ""
}
