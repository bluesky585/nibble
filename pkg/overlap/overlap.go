// Package overlap copies neighboring tokens into Chunk.Context.
// Text and offsets are unchanged, so reconstruct still holds.
package overlap

import (
	"fmt"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

// Prefix sets each chunk's Context to the last n tokens of the previous
// chunk. Existing Context is kept in front. The first chunk is unchanged
// except for a copy.
func Prefix(chunks []chunk.Chunk, tok tokenizer.Tokenizer, n int) ([]chunk.Chunk, error) {
	if tok == nil {
		return nil, fmt.Errorf("tokenizer is required")
	}
	if n < 0 {
		return nil, fmt.Errorf("n must be >= 0, got %d", n)
	}
	if len(chunks) == 0 {
		return nil, nil
	}

	out := make([]chunk.Chunk, len(chunks))
	copy(out, chunks)
	if n == 0 {
		return out, nil
	}

	for i := 1; i < len(out); i++ {
		tail := lastTokens(tok, out[i-1].Text, n)
		if tail == "" {
			continue
		}
		if out[i].Context != "" {
			out[i].Context += tail
		} else {
			out[i].Context = tail
		}
	}
	return out, nil
}

func lastTokens(tok tokenizer.Tokenizer, text string, n int) string {
	parts := tok.Split(text)
	if n >= len(parts) {
		return text
	}
	return tokenizer.Join(parts[len(parts)-n:])
}
