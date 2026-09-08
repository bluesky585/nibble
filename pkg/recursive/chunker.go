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
// exceeds Size is split with the next Level.
type Chunker struct {
	tok   tokenizer.Tokenizer
	size  int
	rules []Level
	hard  tokenchunker.Chunker
}

// New builds a Chunker. Empty rules use DefaultRules.
func New(tok tokenizer.Tokenizer, size int, rules []Level) (Chunker, error) {
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
	return Chunker{tok: tok, size: size, rules: rules, hard: hard}, nil
}

// Chunk splits text using the rule stack.
func (c Chunker) Chunk(text string) ([]chunk.Chunk, error) {
	return c.chunkAt(text, 0, 0)
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
