package tokenizer

import "unicode"

// Word splits on Unicode whitespace. Each run of non-space runes and
// each run of space runes is one token, so Join(Split(text)) == text.
type Word struct{}

// Split returns word and whitespace runs.
func (Word) Split(text string) []string {
	if text == "" {
		return nil
	}

	var parts []string
	var b []rune
	lastSpace := false
	started := false
	for _, r := range text {
		space := unicode.IsSpace(r)
		if !started {
			lastSpace = space
			started = true
		} else if space != lastSpace {
			parts = append(parts, string(b))
			b = b[:0]
			lastSpace = space
		}
		b = append(b, r)
	}
	parts = append(parts, string(b))
	return parts
}

// Count returns the number of word and whitespace runs.
func (Word) Count(text string) int {
	return len(Word{}.Split(text))
}
