package bergamot

import (
	"strings"
	"unicode"
)

func splitSentences(text string) []string {
	var sentences []string
	start := 0
	for offset, current := range text {
		if !isSentenceTerminator(current) {
			continue
		}
		end := offset + len(string(current))
		remainder := text[end:]
		if remainder != "" {
			next, _ := firstRune(remainder)
			if !unicode.IsSpace(next) {
				continue
			}
		}
		if sentence := strings.TrimSpace(text[start:end]); sentence != "" {
			sentences = append(sentences, strings.Clone(sentence))
		}
		start = end
	}
	if sentence := strings.TrimSpace(text[start:]); sentence != "" {
		sentences = append(sentences, strings.Clone(sentence))
	}
	return sentences
}

func isSentenceTerminator(r rune) bool {
	switch r {
	case '.', '!', '?', '\u3002', '\uff01', '\uff1f':
		return true
	default:
		return false
	}
}

func firstRune(text string) (rune, int) {
	for _, r := range text {
		return r, len(string(r))
	}
	return 0, 0
}
