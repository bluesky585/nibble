// Package semantic packs sentences that are similar in embedding space.
package semantic

import (
	"fmt"
	"strings"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/embed"
	"github.com/bluesky585/nibble/pkg/recursive"
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
	// simWindow is how many sentences on each side of a boundary
	// candidate form the two compared groups. 1 compares adjacent pairs,
	// which is noisy when one sentence's wording jitters; larger values
	// average over more sentences.
	simWindow int
	hard      tokenchunker.Chunker
	// fallback re-cuts one over-budget sentence at clause and whitespace
	// delimiters before token windows are the only option left. It shares
	// this chunker's tokenizer and size; see sentencechunker, whose
	// over-budget path uses the same fallback, for why it is built in New.
	fallback recursive.Chunker
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
	fallback, err := recursive.New(tok, size, recursive.FallbackRules())
	if err != nil {
		return Chunker{}, err
	}
	c := Chunker{tok: tok, emb: emb, size: size, minSim: minSim, simWindow: 1, hard: hard, fallback: fallback}
	for _, opt := range opts {
		if err := opt(&c); err != nil {
			return Chunker{}, err
		}
	}
	return c, nil
}

// SimilarityWindow sets how many sentences on each side of a boundary
// candidate are compared. The two group means — one over the window
// before the cut point, one over the window after it — are embedded
// vectors, and their cosine decides the cut.
//
// The default 1 compares adjacent sentence pairs, which is noisy: one
// sentence whose wording jitters can drop a pair below the threshold and
// split a topic that never changed. A window of 2 or 3 averages that
// jitter away while a real topic change still drops the group-to-group
// similarity. Larger windows blur short topics into their neighbors, so
// keep it small. Must be >= 1.
func SimilarityWindow(n int) Option {
	return func(c *Chunker) error {
		if n < 1 {
			return fmt.Errorf("similarity window must be >= 1, got %d", n)
		}
		c.simWindow = n
		return nil
	}
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

	// sim[i] is the group-to-group similarity at cut point i, the point
	// before piece i. A window of 1 compares the adjacent pair, matching
	// the pairwise test exactly; a larger window compares the mean
	// vectors of the w sentences on each side. A point without a full
	// window on both sides — near either end of the document — is not
	// evaluated: a cut needs the window's worth of evidence on both
	// sides, and a partial window would compare a diluted group against
	// a whole one. The value 2 is above the [0, 1] range, so those
	// points never become candidates.
	sim := make([]float64, len(pieces))
	w := c.simWindow
	for i := 1; i < len(pieces); i++ {
		lo, hi := i-w, i+w
		if lo < 0 || hi > len(pieces) {
			sim[i] = 2
			continue
		}
		sim[i] = embed.Cosine32(meanVec(vecs[lo:i]), meanVec(vecs[i:hi]))
	}

	// A real boundary dips below the threshold for a run of consecutive
	// points, because the windows on both sides straddle the change and
	// each mixes the two topics. One change is one cut, so the cut goes
	// at the run's deepest point rather than at every point in it. That
	// also keeps w=1 from sawing a gradually drifting topic into
	// per-sentence pieces: consecutive dissimilar pairs become one cut
	// at the pair least like each other.
	boundary := make([]bool, len(pieces))
	inRun := false
	deepest := 0
	for i := 1; i <= len(pieces); i++ {
		if i < len(pieces) && sim[i] < c.minSim {
			if !inRun || sim[i] < sim[deepest] {
				deepest = i
			}
			inRun = true
			continue
		}
		if inRun {
			boundary[deepest] = true
			inRun = false
		}
	}

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
			if tokens+n > c.size || boundary[i] {
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

// hardSplit cuts one oversized sentence, shifting the piece offsets from
// the sentence into the document. The first attempt uses the fallback
// chunker — clauses, then whitespace, then tokens — so a sentence with a
// comma is cut at the comma instead of mid-word; only a sentence with no
// finer delimiter reaches the token windows unchanged. The token windows
// stay reachable through the fallback's own token level.
func (c Chunker) hardSplit(p split.Piece) ([]chunk.Chunk, error) {
	chunks, err := c.fallback.Chunk(p.Text)
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

// meanVec returns the component-wise mean of vectors. All vectors have
// the same length because one embedder produced them.
func meanVec(vs [][]float32) []float32 {
	m := make([]float32, len(vs[0]))
	for _, v := range vs {
		for i := range v {
			m[i] += v[i]
		}
	}
	n := float32(len(vs))
	for i := range m {
		m[i] /= n
	}
	return m
}
