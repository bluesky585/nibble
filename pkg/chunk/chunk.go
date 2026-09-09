// Package chunk defines the unit of split text.
package chunk

import (
	"fmt"
	"unicode/utf8"
)

// Chunk is a contiguous slice of a document.
//
// Start and End are rune offsets into the original text and form a
// half-open interval [Start, End). The number of runes in Text must
// equal End - Start.
type Chunk struct {
	Text       string `json:"text"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
	TokenCount int    `json:"token_count"`
	// Context is extra text for retrieval (for example a table header).
	// It is not part of Text and is ignored by reconstruct checks.
	Context string `json:"context,omitempty"`
}

// New builds a Chunk and checks its invariants.
func New(text string, start, end, tokenCount int) (Chunk, error) {
	if start < 0 {
		return Chunk{}, fmt.Errorf("start must be >= 0, got %d", start)
	}
	if end < start {
		return Chunk{}, fmt.Errorf("end must be >= start, got start=%d end=%d", start, end)
	}
	if tokenCount < 0 {
		return Chunk{}, fmt.Errorf("token_count must be >= 0, got %d", tokenCount)
	}
	runes := utf8.RuneCountInString(text)
	if runes != end-start {
		return Chunk{}, fmt.Errorf(
			"text rune count %d must equal end-start %d",
			runes,
			end-start,
		)
	}
	return Chunk{
		Text:       text,
		Start:      start,
		End:        end,
		TokenCount: tokenCount,
	}, nil
}
