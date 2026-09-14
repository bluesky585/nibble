// Package sentencechunker splits on sentence delimiters, then packs
// sentences into chunks up to a token budget.
package sentencechunker

import (
	"fmt"
	"strings"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/split"
	"github.com/bluesky585/nibble/pkg/tokenchunker"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

// DefaultDelimiters are common sentence endings, including CJK.
var DefaultDelimiters = []string{"。", "！", "？", ".", "!", "?"}

// Chunker splits on sentence delimiters and merges until Size tokens.
// A single sentence longer than Size is cut into token windows, so no
// chunk exceeds Size unless one token does.
type Chunker struct {
	tok    tokenizer.Tokenizer
	size   int
	delims []string
	hard   tokenchunker.Chunker
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
	hard, err := tokenchunker.New(tok, size, 0)
	if err != nil {
		return Chunker{}, err
	}
	return Chunker{tok: tok, size: size, delims: delims, hard: hard}, nil
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
		// A sentence that is over budget on its own cannot be packed.
		// Cut it into token windows instead of emitting it whole.
		if n > c.size {
			more, err := c.hardSplit(p)
			if err != nil {
				return nil, err
			}
			out = append(out, more...)
			continue
		}
		buf = append(buf, p)
		tokens += n
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return out, nil
}

// hardSplit cuts one oversized sentence into token windows, shifting the
// window offsets from the sentence into the document.
func (c Chunker) hardSplit(p split.Piece) ([]chunk.Chunk, error) {
	chunks, err := c.hard.Chunk(p.Text)
	if err != nil {
		return nil, err
	}
	if p.Start == 0 {
		return chunks, nil
	}
	out := make([]chunk.Chunk, len(chunks))
	for i, ch := range chunks {
		ch.Start += p.Start
		ch.End += p.Start
		out[i] = ch
	}
	return out, nil
}
