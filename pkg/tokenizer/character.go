package tokenizer

import "unicode/utf8"

// Character treats each Unicode code point as one token.
// The token id is the rune value itself.
type Character struct{}

// Split returns one piece per rune.
func (Character) Split(text string) []string {
	if text == "" {
		return nil
	}
	parts := make([]string, 0, utf8.RuneCountInString(text))
	for _, r := range text {
		parts = append(parts, string(r))
	}
	return parts
}

// Encode maps each rune in text to its code point.
func (Character) Encode(text string) []int {
	n := utf8.RuneCountInString(text)
	ids := make([]int, 0, n)
	for _, r := range text {
		ids = append(ids, int(r))
	}
	return ids
}

// Decode concatenates token ids as runes.
func (Character) Decode(tokens []int) string {
	runes := make([]rune, len(tokens))
	for i, id := range tokens {
		runes[i] = rune(id)
	}
	return string(runes)
}

// Count returns the number of runes in text.
func (Character) Count(text string) int {
	return utf8.RuneCountInString(text)
}
