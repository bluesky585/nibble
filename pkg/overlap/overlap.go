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
	out, err := copyForOverlap(chunks, tok, n)
	if err != nil || n == 0 || len(out) == 0 {
		return out, err
	}

	for i := 1; i < len(out); i++ {
		out[i].Context = appendContext(out[i].Context, lastTokens(tok, out[i-1].Text, n))
	}
	return out, nil
}

// Suffix sets each chunk's Context to the first n tokens of the next
// chunk. Existing Context is kept in front. The last chunk is unchanged
// except for a copy.
func Suffix(chunks []chunk.Chunk, tok tokenizer.Tokenizer, n int) ([]chunk.Chunk, error) {
	out, err := copyForOverlap(chunks, tok, n)
	if err != nil || n == 0 || len(out) == 0 {
		return out, err
	}
	for i := 0; i < len(out)-1; i++ {
		out[i].Context = appendContext(out[i].Context, firstTokens(tok, out[i+1].Text, n))
	}
	return out, nil
}

func copyForOverlap(chunks []chunk.Chunk, tok tokenizer.Tokenizer, n int) ([]chunk.Chunk, error) {
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
	return out, nil
}

func appendContext(existing, extra string) string {
	if extra == "" {
		return existing
	}
	if existing == "" {
		return extra
	}
	return existing + extra
}

func lastTokens(tok tokenizer.Tokenizer, text string, n int) string {
	parts := tok.Split(text)
	if n >= len(parts) {
		return text
	}
	return tokenizer.Join(parts[len(parts)-n:])
}

func firstTokens(tok tokenizer.Tokenizer, text string, n int) string {
	parts := tok.Split(text)
	if n >= len(parts) {
		return text
	}
	return tokenizer.Join(parts[:n])
}
