// Package recursive splits text from coarse structure down to tokens.
package recursive

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/split"
	"github.com/bluesky585/nibble/pkg/tokenchunker"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

// Chunker packs delimiter pieces up to Size tokens. A piece that still
// exceeds Size is split with the next Level. Overlap repeats the tail
// of each chunk at the head of the next, in Text itself, the way the
// token chunker widens a window.
type Chunker struct {
	tok     tokenizer.Tokenizer
	size    int
	rules   []Level
	overlap int
	hard    tokenchunker.Chunker
}

// An Option changes a Chunker from its defaults. New takes them last.
type Option func(*Chunker) error

// Overlap sets how many tokens of the previous chunk's tail the next
// chunk repeats in its own Text. Zero, the default, leaves chunks
// contiguous so joining them reconstructs the input; a positive overlap
// breaks that guarantee the same way the token chunker's overlap does —
// the repeated run is text the chunk does not own, but it is a slice of
// the source, offsets stay exact, and the last chunk still ends at the
// end of the input. It must be >= 0 and < size.
func Overlap(n int) Option {
	return func(c *Chunker) error {
		if n < 0 {
			return fmt.Errorf("overlap must be >= 0, got %d", n)
		}
		if n >= c.size {
			return fmt.Errorf("overlap must be < size, got overlap=%d size=%d", n, c.size)
		}
		c.overlap = n
		return nil
	}
}

// New builds a Chunker. Empty rules use DefaultRules.
func New(tok tokenizer.Tokenizer, size int, rules []Level, opts ...Option) (Chunker, error) {
	if tok == nil {
		return Chunker{}, fmt.Errorf("tokenizer is required")
	}
	if size <= 0 {
		return Chunker{}, fmt.Errorf("size must be > 0, got %d", size)
	}
	if len(rules) == 0 {
		rules = DefaultRules()
	} else {
		rules = append([]Level(nil), rules...)
		for _, l := range rules {
			if err := l.validate(); err != nil {
				return Chunker{}, err
			}
		}
	}
	hard, err := tokenchunker.New(tok, size, 0)
	if err != nil {
		return Chunker{}, err
	}
	c := Chunker{tok: tok, size: size, rules: rules, hard: hard}
	for _, opt := range opts {
		if err := opt(&c); err != nil {
			return Chunker{}, err
		}
	}
	return c, nil
}

// Chunk splits text using the rule stack.
func (c Chunker) Chunk(text string) ([]chunk.Chunk, error) {
	chunks, err := c.chunkAt(text, 0, 0)
	if err != nil {
		return nil, err
	}
	chunks = mergeBlank(chunks)
	// Overlap is applied last, on the final sequence, so the run each
	// chunk repeats belongs to its actual neighbor and not to a chunk a
	// merge later absorbed.
	if c.overlap > 0 {
		chunks = applyOverlap(chunks, c.tok, c.overlap)
	}
	return chunks, nil
}

// applyOverlap widens each chunk after the first to repeat the last n
// tokens of the previous chunk's tail, taken from the source text the
// offsets point at. Because token pieces concatenate to their text, the
// run is the tail of the previous chunk's own text: it ends where the
// next chunk began, and no token can straddle the two chunks. Start
// moves back, the repeated text is prepended, and TokenCount grows by
// the tokens in the repeated run as measured on its own — a recount
// made on a different string than the chunker's pack-time tally, which
// is the same gap checkBudget already documents.
func applyOverlap(chunks []chunk.Chunk, tok tokenizer.Tokenizer, n int) []chunk.Chunk {
	if len(chunks) == 0 {
		return chunks
	}
	out := make([]chunk.Chunk, len(chunks))
	copy(out, chunks)
	for i := 1; i < len(out); i++ {
		parts := tok.Split(out[i-1].Text)
		if len(parts) == 0 {
			continue
		}
		first := len(parts) - n
		if first < 0 {
			first = 0
		}
		repeat := tokenizer.Join(parts[first:])
		runes := utf8.RuneCountInString(repeat)
		out[i].Text = repeat + out[i].Text
		out[i].Start -= runes
		out[i].TokenCount += tok.Count(repeat)
	}
	return out
}

// mergeBlank absorbs chunks that hold only whitespace into a
// neighboring chunk. Such a chunk appears when a paragraph exactly
// fills the budget: its trailing blank-line separator becomes its own
// piece, overflows to the next group, and hard-splits as a chunk with
// no content in it. It is merged rather than dropped so reconstruct
// still holds; the neighbor may then exceed Size by the separator's
// width. Blanks before the first chunk with content merge forward into
// it, in order. If every chunk is blank the input is left alone:
// returning nothing would hide it.
func mergeBlank(chunks []chunk.Chunk) []chunk.Chunk {
	hasContent := false
	for _, ch := range chunks {
		if strings.TrimSpace(ch.Text) != "" {
			hasContent = true
			break
		}
	}
	if !hasContent {
		return chunks
	}

	var out []chunk.Chunk
	var carry strings.Builder
	carryStart := 0
	carryTokens := 0
	for _, ch := range chunks {
		if strings.TrimSpace(ch.Text) == "" {
			if len(out) == 0 {
				// No chunk with content to merge into yet, so the blank
				// waits to become the prefix of the next one.
				if carry.Len() == 0 {
					carryStart = ch.Start
				}
				carry.WriteString(ch.Text)
				carryTokens += ch.TokenCount
				continue
			}
			prev := &out[len(out)-1]
			prev.Text += ch.Text
			prev.End = ch.End
			prev.TokenCount += ch.TokenCount
			continue
		}
		if carry.Len() > 0 {
			ch.Text = carry.String() + ch.Text
			ch.Start = carryStart
			ch.TokenCount += carryTokens
			carry.Reset()
			carryTokens = 0
		}
		out = append(out, ch)
	}
	return out
}

func (c Chunker) chunkAt(text string, level, offset int) ([]chunk.Chunk, error) {
	if text == "" {
		return nil, nil
	}

	n := c.tok.Count(text)
	runes := utf8.RuneCountInString(text)
	if n <= c.size {
		ch, err := chunk.New(text, offset, offset+runes, n)
		if err != nil {
			return nil, err
		}
		return []chunk.Chunk{ch}, nil
	}

	if level >= len(c.rules) || c.rules[level].Token {
		return c.hardSplit(text, offset)
	}

	rule := c.rules[level]
	pieces, err := split.Text(text, split.Options{
		Delimiters: rule.Delimiters,
		Attach:     rule.Attach,
	})
	if err != nil {
		return nil, err
	}
	if len(pieces) <= 1 {
		return c.chunkAt(text, level+1, offset)
	}

	var out []chunk.Chunk
	for _, group := range pack(pieces, c.tok, c.size) {
		gText, gTokens := joinGroup(group, c.tok)
		gOffset := offset + group[0].Start
		if gTokens <= c.size {
			ch, err := chunk.New(gText, gOffset, offset+group[len(group)-1].End, gTokens)
			if err != nil {
				return nil, err
			}
			out = append(out, ch)
			continue
		}
		more, err := c.chunkAt(gText, level+1, gOffset)
		if err != nil {
			return nil, err
		}
		out = append(out, more...)
	}
	return out, nil
}

func (c Chunker) hardSplit(text string, offset int) ([]chunk.Chunk, error) {
	chunks, err := c.hard.Chunk(text)
	if err != nil {
		return nil, err
	}
	if offset == 0 {
		return chunks, nil
	}
	out := make([]chunk.Chunk, len(chunks))
	for i, ch := range chunks {
		ch.Start += offset
		ch.End += offset
		out[i] = ch
	}
	return out, nil
}

func pack(pieces []split.Piece, tok tokenizer.Tokenizer, size int) [][]split.Piece {
	var groups [][]split.Piece
	buf := make([]split.Piece, 0, 4)
	tokens := 0
	flush := func() {
		if len(buf) == 0 {
			return
		}
		g := make([]split.Piece, len(buf))
		copy(g, buf)
		groups = append(groups, g)
		buf = buf[:0]
		tokens = 0
	}
	for _, p := range pieces {
		n := tok.Count(p.Text)
		if len(buf) > 0 && tokens+n > size {
			flush()
		}
		buf = append(buf, p)
		tokens += n
	}
	flush()
	return groups
}

func joinGroup(group []split.Piece, tok tokenizer.Tokenizer) (string, int) {
	var b strings.Builder
	n := 0
	for _, p := range group {
		b.WriteString(p.Text)
		n += tok.Count(p.Text)
	}
	return b.String(), n
}
