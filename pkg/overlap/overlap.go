// Package overlap copies neighboring tokens into Chunk.Context.
// Text and offsets are unchanged, so reconstruct still holds.
//
// The name is about what gets copied, not about the -overlap flag, which
// this package does not implement. -overlap repeats text by widening each
// token window, so the repeat lives in Chunk.Text (see pkg/tokenchunker).
// Here nothing is repeated: Context receives a preview of the neighbor
// while Text stays a slice of the source. Prefix implements
// -context-mode prefix and Suffix implements -context-mode suffix.
//
// Merge is the exception, and it is opt-in: it folds the copied context
// into Text itself, so Text is no longer a slice of the source and the
// reconstruct guarantee does not hold. Its doc comment says when to
// prefer it.
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

// Mode names where Merge takes its context from.
type Mode string

const (
	// PrefixMode takes context from the end of the previous chunk.
	PrefixMode Mode = "prefix"
	// SuffixMode takes context from the start of the next chunk.
	SuffixMode Mode = "suffix"
	// JustifiedMode takes context from both neighbors: the first chunk
	// gets only suffix, the last only prefix, middle chunks both.
	JustifiedMode Mode = "justified"
)

// Merge folds the neighbor context into Text itself, the way an overlap
// refinery with merge enabled does: the chunk an embedder or a reader
// sees carries its neighbor's tail (or head) inside the text, not beside
// it.
//
// This is the one function in this package that breaks the reconstruct
// guarantee. Text is no longer a slice of the source document, so
// joining chunks does not restore the input, and Start/End keep naming
// the slice the original text came from rather than the merged string.
// A caller that needs reconstruct uses Prefix or Suffix instead; a
// caller that wants every embedded chunk to hold its own bridge to the
// neighbors — retrieval where chunks are consumed independently — uses
// Merge.
//
// TokenCount grows by the context's token count, since Text now holds
// it. Existing Context is kept in front of the added overlap, matching
// Prefix and Suffix. The input is copied, not modified.
func Merge(chunks []chunk.Chunk, tok tokenizer.Tokenizer, n int, mode Mode) ([]chunk.Chunk, error) {
	switch mode {
	case PrefixMode, SuffixMode, JustifiedMode:
	default:
		return nil, fmt.Errorf("unknown mode %q, want prefix, suffix, or justified", mode)
	}
	out, err := copyForOverlap(chunks, tok, n)
	if err != nil || n == 0 || len(out) == 0 {
		return out, err
	}

	// Contexts are computed from the unmodified chunks first, so no
	// function reads a text it has already merged into.
	prefix := make([]string, len(out))
	suffix := make([]string, len(out))
	for i := range out {
		if mode == PrefixMode || mode == JustifiedMode {
			if i > 0 {
				prefix[i] = lastTokens(tok, chunks[i-1].Text, n)
			}
		}
		if mode == SuffixMode || mode == JustifiedMode {
			if i < len(out)-1 {
				suffix[i] = firstTokens(tok, chunks[i+1].Text, n)
			}
		}
	}

	for i := range out {
		context := prefix[i] + suffix[i]
		if context == "" {
			continue
		}
		out[i].Context = appendContext(out[i].Context, context)
		out[i].Text = prefix[i] + out[i].Text + suffix[i]
		out[i].TokenCount += tok.Count(context)
	}
	return out, nil
}
