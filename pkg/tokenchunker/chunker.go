// Package tokenchunker splits text into windows of a fixed token count.
package tokenchunker

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

// Chunker splits text into windows of Size tokens.
// Overlap is the number of tokens shared with the previous window.
type Chunker struct {
	tok     tokenizer.Tokenizer
	size    int
	overlap int
}

// New builds a Chunker.
func New(tok tokenizer.Tokenizer, size, overlap int) (Chunker, error) {
	if tok == nil {
		return Chunker{}, fmt.Errorf("tokenizer is required")
	}
	if size <= 0 {
		return Chunker{}, fmt.Errorf("size must be > 0, got %d", size)
	}
	if overlap < 0 {
		return Chunker{}, fmt.Errorf("overlap must be >= 0, got %d", overlap)
	}
	if overlap >= size {
		return Chunker{}, fmt.Errorf("overlap must be < size, got overlap=%d size=%d", overlap, size)
	}
	return Chunker{tok: tok, size: size, overlap: overlap}, nil
}

// Chunk splits text into token windows.
func (c Chunker) Chunk(text string) ([]chunk.Chunk, error) {
	// A rune tokenizer's windows are plain substrings: measuring is a
	// rune walk with no piece slice to build, and a window is text[
	// lo:hi] rather than a join of single-rune strings. The result is
	// identical to the general path below — same chunks, same offsets,
	// same counts — without the per-rune string the general path makes
	// the character tokenizer allocate and the join then throws away.
	if rt, ok := c.tok.(tokenizer.Runes); ok && rt.IsRunes() {
		return c.chunkRunes(text)
	}
	return c.chunkPieces(text)
}

// chunkRunes is Chunk for a tokenizer whose tokens are runes. Windows
// start every step runes and span size runes — the same arithmetic the
// general path runs over piece indexes — but a window here is a slice
// of the original text, and finding its bounds is a rune walk with
// nothing allocated per rune.
func (c Chunker) chunkRunes(text string) ([]chunk.Chunk, error) {
	step := c.size - c.overlap
	// Rune starts in bytes: starts[i] is where rune i begins. The final
	// entry is the byte length, the exclusive end of the last rune.
	n := utf8.RuneCountInString(text)
	starts := make([]int, 0, n+1)
	for b := range text {
		starts = append(starts, b)
	}
	starts = append(starts, len(text))

	out := make([]chunk.Chunk, 0, (n+step-1)/step)
	for lo := 0; lo < n; lo += step {
		hi := lo + c.size
		if hi > n {
			hi = n
		}
		if starts[hi] > starts[lo] {
			ch, err := chunk.New(text[starts[lo]:starts[hi]], lo, hi, hi-lo)
			if err != nil {
				return nil, err
			}
			out = append(out, ch)
		}
		if hi == n {
			break
		}
	}
	return out, nil
}

// chunkPieces is the general path: split into pieces, then window over
// the piece slice. Any tokenizer works here, including ones whose
// pieces are not runes and can be empty.
func (c Chunker) chunkPieces(text string) ([]chunk.Chunk, error) {
	parts := c.tok.Split(text)
	if len(parts) == 0 {
		return nil, nil
	}

	starts := make([]int, len(parts)+1)
	for i, p := range parts {
		starts[i+1] = starts[i] + utf8.RuneCountInString(p)
	}

	step := c.size - c.overlap
	out := make([]chunk.Chunk, 0, (len(parts)+step-1)/step)
	for i := 0; i < len(parts); i += step {
		end := i + c.size
		if end > len(parts) {
			end = len(parts)
		}
		// A piece can be empty. That is a token that ended inside a
		// character: the tokenizer reports it as an empty piece so the
		// character is carried whole by the piece that completes it. When a
		// single character is wider than the whole budget, every piece of a
		// window can be empty, so the window covers no runes at all. That
		// window is not a chunk: it is empty text carrying a token count,
		// with an empty range, which nothing can retrieve on. Skipping it
		// keeps the chunks contiguous, because the next window starts where
		// this one did, and the character it was waiting for is emitted by
		// that next window.
		if starts[end] > starts[i] {
			piece := strings.Join(parts[i:end], "")
			ch, err := chunk.New(piece, starts[i], starts[end], end-i)
			if err != nil {
				return nil, err
			}
			out = append(out, ch)
		}
		if end == len(parts) {
			break
		}
	}
	return out, nil
}
