// Package codechunker splits Go source on top-level declarations.
package codechunker

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/tokenchunker"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

// Chunker splits a Go file into declaration-sized windows.
// Source that does not parse falls back to token windows.
type Chunker struct {
	tok  tokenizer.Tokenizer
	size int
	hard tokenchunker.Chunker
}

// New builds a Chunker.
func New(tok tokenizer.Tokenizer, size int) (Chunker, error) {
	if tok == nil {
		return Chunker{}, fmt.Errorf("tokenizer is required")
	}
	if size <= 0 {
		return Chunker{}, fmt.Errorf("size must be > 0, got %d", size)
	}
	hard, err := tokenchunker.New(tok, size, 0)
	if err != nil {
		return Chunker{}, err
	}
	return Chunker{tok: tok, size: size, hard: hard}, nil
}

// Chunk splits Go source. Offsets are rune indexes into text.
func (c Chunker) Chunk(text string) ([]chunk.Chunk, error) {
	if text == "" {
		return nil, nil
	}

	pieces := c.declPieces(text)
	if len(pieces) == 0 {
		return c.hardSplit(text, 0)
	}

	var out []chunk.Chunk
	var buf []piece
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
		joined := b.String()
		start := buf[0].Start
		end := buf[len(buf)-1].End
		if n <= c.size {
			ch, err := chunk.New(joined, start, end, n)
			if err != nil {
				return err
			}
			out = append(out, ch)
		} else {
			more, err := c.hardSplit(joined, start)
			if err != nil {
				return err
			}
			out = append(out, more...)
		}
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

type piece struct {
	Text  string
	Start int
	End   int
}

func (c Chunker) declPieces(text string) []piece {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "src.go", text, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil
	}

	cuts := []int{0, len(text)}
	for _, d := range f.Decls {
		start, _ := declSpan(d)
		off := fset.Position(start).Offset
		if off > 0 && off < len(text) {
			cuts = append(cuts, off)
		}
	}
	sort.Ints(cuts)
	uniq := cuts[:0]
	prev := -1
	for _, x := range cuts {
		if x != prev {
			uniq = append(uniq, x)
			prev = x
		}
	}

	out := make([]piece, 0, len(uniq)-1)
	for i := 0; i < len(uniq)-1; i++ {
		a, b := uniq[i], uniq[i+1]
		if a == b {
			continue
		}
		seg := text[a:b]
		out = append(out, piece{
			Text:  seg,
			Start: utf8.RuneCountInString(text[:a]),
			End:   utf8.RuneCountInString(text[:b]),
		})
	}
	return out
}

func declSpan(d ast.Decl) (token.Pos, token.Pos) {
	start, end := d.Pos(), d.End()
	switch x := d.(type) {
	case *ast.FuncDecl:
		if x.Doc != nil {
			start = x.Doc.Pos()
		}
	case *ast.GenDecl:
		if x.Doc != nil {
			start = x.Doc.Pos()
		}
	}
	return start, end
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
