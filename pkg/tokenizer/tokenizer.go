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

// Runes is an optional interface a Tokenizer may implement when its
// tokens are exactly Unicode code points: Split returns one piece per
// rune, each piece one rune long, and Count is RuneCountInString. The
// character tokenizer is the only implementation.
//
// Chunkers that build windows over pieces can take a fast path on it:
// counting and offsetting a rune tokenizer is a rune count, not a walk
// of a piece slice, and rejoining a window is a substring of the
// original text rather than a join of single-rune strings. The type
// assertion is local and checked — a tokenizer that does not implement
// Runes takes the general path unchanged.
type Runes interface {
	Tokenizer
	// IsRunes reports that this tokenizer's tokens are runes. Always
	// true where the assertion succeeds; it exists so the fast path can
	// be written as a plain capability check.
	IsRunes() bool
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
