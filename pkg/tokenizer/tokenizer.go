// Package tokenizer counts and maps text for chunk size limits.
package tokenizer

import "strings"

// Tokenizer is the ruler used to measure and split text.
//
// Split must return pieces that concatenate to the original text.
// Count(text) must equal len(Split(text)).
type Tokenizer interface {
	Split(text string) []string
	Count(text string) int
}

// CountBatch counts each text in texts. The results line up with the
// inputs by index, and counts[i] must equal Count(texts[i]).
//
// Chunkers that measure many pieces at once call this instead of Count in
// a loop, so an implementation with a cheaper batch path can use it. The
// default implementation loops over Count, which is exact for every
// tokenizer; an implementation only overrides it when looping is
// measurably slower than one batch call.
func CountBatch(tok Tokenizer, texts []string) []int {
	counts := make([]int, len(texts))
	for i, t := range texts {
		counts[i] = tok.Count(t)
	}
	return counts
}

// Join concatenates token pieces. It is the inverse of a correct Split.
func Join(parts []string) string {
	return strings.Join(parts, "")
}
