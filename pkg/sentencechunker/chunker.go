// Package sentencechunker splits on sentence delimiters, then packs
// sentences into chunks up to a token budget.
package sentencechunker

import (
	"fmt"
	"strings"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/split"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

// DefaultDelimiters are common sentence endings, including CJK.
var DefaultDelimiters = []string{"。", "！", "？", ".", "!", "?"}

// Chunker splits on sentence delimiters and merges until Size tokens.
// A single sentence longer than Size is emitted whole.
type Chunker struct {
	tok    tokenizer.Tokenizer
	size   int
	delims []string
}

// New builds a Chunker. delims defaults to DefaultDelimiters when empty.
func New(tok tokenizer.Tokenizer, size int, delims []string) (Chunker, error) {
	if tok == nil {
		return Chunker{}, fmt.Errorf("tokenizer is required")
	}
	if size <= 0 {
		return Chunker{}, fmt.Errorf("size must be > 0, got %d", size)
	}
	if len(delims) == 0 {
		delims = append([]string(nil), DefaultDelimiters...)
	}
	return Chunker{tok: tok, size: size, delims: delims}, nil
}

// Chunk splits text into sentence-packed windows.
func (c Chunker) Chunk(text string) ([]chunk.Chunk, error) {
	pieces, err := split.Text(text, split.Options{
		Delimiters: c.delims,
		Attach:     split.AttachPrev,
	})
	if err != nil {
		return nil, err
	}
	if len(pieces) == 0 {
		return nil, nil
	}

	var out []chunk.Chunk
	var buf []split.Piece
	tokens := 0

	flush := func() error {
		if len(buf) == 0 {
			return nil
		}
		var b strings.Builder
		n := 0
		for _, p := range buf {
			b.WriteString(p.Text)
			n += c.tok.Count(p.Text)
		}
		ch, err := chunk.New(b.String(), buf[0].Start, buf[len(buf)-1].End, n)
		if err != nil {
			return err
		}
		out = append(out, ch)
		buf = buf[:0]
		tokens = 0
		return nil
	}

	for _, p := range pieces {
		n := c.tok.Count(p.Text)
		if len(buf) > 0 && tokens+n > c.size {
			if err := flush(); err != nil {
				return nil, err
			}
		}
		buf = append(buf, p)
		tokens += n
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return out, nil
}
