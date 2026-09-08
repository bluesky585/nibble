// Package tokenizer counts and maps text for chunk size limits.
package tokenizer

// Tokenizer is the ruler used to measure and split text.
//
// Encode and Decode must round-trip valid Unicode text.
// Count(text) must equal len(Encode(text)).
type Tokenizer interface {
	Encode(text string) []int
	Decode(tokens []int) string
	Count(text string) int
}
