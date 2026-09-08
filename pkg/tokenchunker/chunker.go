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
		piece := strings.Join(parts[i:end], "")
		ch, err := chunk.New(piece, starts[i], starts[end], end-i)
		if err != nil {
			return nil, err
		}
		out = append(out, ch)
		if end == len(parts) {
			break
		}
	}
	return out, nil
}
