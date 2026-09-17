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
	// minRunes is passed to split.Text: pieces shorter than this merge
	// into the next piece. 0 keeps every delimiter cut, which is right
	// for CJK, where a 2-rune sentence is complete. A Latin-script
	// caller may set it to absorb abbreviation fragments ("e." out of
	// "e.g."); a value that is too high merges whole sentences together,
	// so it is opt-in per caller rather than a default.
	minRunes int
	hard     tokenchunker.Chunker
}

// Option configures a Chunker beyond the required arguments.
type Option func(*Chunker) error

// New builds a Chunker. delims defaults to DefaultDelimiters when empty.
// Options are applied in order after the defaults are set.
func New(tok tokenizer.Tokenizer, size int, delims []string, opts ...Option) (Chunker, error) {
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
	c := Chunker{tok: tok, size: size, delims: delims, hard: hard}
	for _, opt := range opts {
		if err := opt(&c); err != nil {
			return Chunker{}, err
		}
	}
	return c, nil
}

// MinRunes merges sentence pieces shorter than n runes into the next
// piece before packing, absorbing delimiter fragments such as the "e."
// that "e.g. this" yields under ".". The last piece may stay short.
//
// The zero value keeps every cut, which is the right default for CJK:
// there a 2-rune sentence is complete, and merging it with its neighbor
// destroys a real boundary. A Latin-script caller with abbreviation-heavy
// text opts in with a small value, typically 4 to 12; too high a value
// merges real sentences together. Counts are runes, not bytes.
func MinRunes(n int) Option {
	return func(c *Chunker) error {
		if n < 0 {
			return fmt.Errorf("min runes must be >= 0, got %d", n)
		}
		c.minRunes = n
		return nil
	}
}

// Chunk splits text into sentence-packed windows.
func (c Chunker) Chunk(text string) ([]chunk.Chunk, error) {
	pieces, err := split.Text(text, split.Options{
		Delimiters: c.delims,
		Attach:     split.AttachPrev,
		MinRunes:   c.minRunes,
	})
	if err != nil {
		return nil, err
	}
	if len(pieces) == 0 {
		return nil, nil
	}

	var out []chunk.Chunk
	// buf holds indexes into pieces, not the pieces themselves, so flush
	// can sum the precomputed counts instead of counting text again.
	var buf []int
	tokens := 0

	// Every piece is measured before the packing loop starts, in one call
	// per input rather than one call per piece inside flush. The counts
	// are reused by both the packing decision and the emitted chunks.
	counts := tokenizer.CountBatch(c.tok, split.Texts(pieces))

	flush := func() error {
		if len(buf) == 0 {
			return nil
		}
		var b strings.Builder
		n := 0
		for _, i := range buf {
			b.WriteString(pieces[i].Text)
			n += counts[i]
		}
		first, last := pieces[buf[0]], pieces[buf[len(buf)-1]]
		ch, err := chunk.New(b.String(), first.Start, last.End, n)
		if err != nil {
			return err
		}
		out = append(out, ch)
		buf = buf[:0]
		tokens = 0
		return nil
	}

	for i, p := range pieces {
		n := counts[i]
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
		buf = append(buf, i)
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
