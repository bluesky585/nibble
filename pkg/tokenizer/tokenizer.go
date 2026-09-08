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

// Join concatenates token pieces. It is the inverse of a correct Split.
func Join(parts []string) string {
	return strings.Join(parts, "")
}
