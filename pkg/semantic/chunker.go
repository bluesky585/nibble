// Package semantic packs sentences that are similar in embedding space.
package semantic

import (
	"fmt"
	"strings"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/embed"
	"github.com/bluesky585/nibble/pkg/sentencechunker"
	"github.com/bluesky585/nibble/pkg/split"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

const defaultMinSim = 0.5

// Chunker splits on sentences, then starts a new chunk when the next
// sentence is dissimilar or the token budget is full.
type Chunker struct {
	tok    tokenizer.Tokenizer
	emb    embed.Embedder
	size   int
	minSim float64
}

// New builds a Chunker. minSim 0 uses 0.5. Similarity is cosine in [0, 1]
// for the hashing embedder (non-negative counts).
func New(tok tokenizer.Tokenizer, emb embed.Embedder, size int, minSim float64) (Chunker, error) {
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
	return Chunker{tok: tok, emb: emb, size: size, minSim: minSim}, nil
}

// Chunk splits text. Sentence embeddings are requested in one batch.
func (c Chunker) Chunk(text string) ([]chunk.Chunk, error) {
	pieces, err := split.Text(text, split.Options{
		Delimiters: sentencechunker.DefaultDelimiters,
		Attach:     split.AttachPrev,
	})
	if err != nil {
		return nil, err
	}
	if len(pieces) == 0 {
		return nil, nil
	}

	texts := make([]string, len(pieces))
	for i, p := range pieces {
		texts[i] = p.Text
	}
	vecs, err := c.emb.Embed(texts)
	if err != nil {
		return nil, err
	}
	if len(vecs) != len(pieces) {
		return nil, fmt.Errorf("embedder returned %d vectors for %d sentences", len(vecs), len(pieces))
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

	for i, p := range pieces {
		n := c.tok.Count(p.Text)
		if len(buf) > 0 {
			if tokens+n > c.size || embed.Cosine(vecs[i-1], vecs[i]) < c.minSim {
				if err := flush(); err != nil {
					return nil, err
				}
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
