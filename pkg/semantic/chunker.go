// Package semantic packs sentences that are similar in embedding space.
package semantic

import (
	"fmt"
	"strings"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/embed"
	"github.com/bluesky585/nibble/pkg/sentencechunker"
	"github.com/bluesky585/nibble/pkg/split"
	"github.com/bluesky585/nibble/pkg/tokenchunker"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

const defaultMinSim = 0.5

// Chunker splits on sentences, then starts a new chunk when the next
// sentence is dissimilar or the token budget is full. A sentence over
// budget on its own is cut into token windows.
type Chunker struct {
	tok    tokenizer.Tokenizer
	emb    embed.Embedder
	size   int
	minSim float64
	// minRunes is passed to the sentence split; see
	// sentencechunker.MinRunes for why it is opt-in rather than a default.
	minRunes int
	hard     tokenchunker.Chunker
}

// Option configures a Chunker beyond the required arguments.
type Option func(*Chunker) error

// New builds a Chunker. minSim 0 uses 0.5. Similarity is cosine in [0, 1]
// for the hashing embedder (non-negative counts).
func New(tok tokenizer.Tokenizer, emb embed.Embedder, size int, minSim float64, opts ...Option) (Chunker, error) {
	if tok == nil {
		return Chunker{}, fmt.Errorf("tokenizer is required")
	}
	if emb == nil {
		return Chunker{}, fmt.Errorf("embedder is required")
	}
	if size <= 0 {
		return Chunker{}, fmt.Errorf("size must be > 0, got %d", size)
	}
	if minSim == 0 {
		minSim = defaultMinSim
	}
	if minSim < 0 || minSim > 1 {
		return Chunker{}, fmt.Errorf("minSim must be in [0, 1], got %v", minSim)
	}
	hard, err := tokenchunker.New(tok, size, 0)
	if err != nil {
		return Chunker{}, err
	}
	c := Chunker{tok: tok, emb: emb, size: size, minSim: minSim, hard: hard}
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
// there a 2-rune sentence is complete. See sentencechunker.MinRunes,
// which this passes through to the shared sentence split.
func MinRunes(n int) Option {
	return func(c *Chunker) error {
		if n < 0 {
			return fmt.Errorf("min runes must be >= 0, got %d", n)
		}
		c.minRunes = n
		return nil
	}
}

// Chunk splits text. Sentence embeddings are requested in one batch.
func (c Chunker) Chunk(text string) ([]chunk.Chunk, error) {
	pieces, err := split.Text(text, split.Options{
		Delimiters: sentencechunker.DefaultDelimiters,
		Attach:     split.AttachPrev,
		MinRunes:   c.minRunes,
	})
	if err != nil {
		return nil, err
	}
	if len(pieces) == 0 {
		return nil, nil
	}

	texts := split.Texts(pieces)
	vecs, err := c.emb.Embed(texts)
	if err != nil {
		return nil, err
	}
	if len(vecs) != len(pieces) {
		return nil, fmt.Errorf("embedder returned %d vectors for %d sentences", len(vecs), len(pieces))
	}
	// Measured once for the whole input, like the embeddings above: both
	// the packing decision and the emitted chunks read these counts.
	counts := tokenizer.CountBatch(c.tok, texts)

	var out []chunk.Chunk
	// buf holds indexes into pieces, not the pieces themselves, so flush
	// can sum the precomputed counts instead of counting text again.
	var buf []int
	tokens := 0

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
		if len(buf) > 0 {
			if tokens+n > c.size || embed.Cosine(vecs[i-1], vecs[i]) < c.minSim {
				if err := flush(); err != nil {
					return nil, err
				}
			}
		}
		// A sentence over budget on its own cannot be packed. Cut it
		// into token windows instead of emitting it whole.
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
