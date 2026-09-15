// Package codechunker splits source code on top-level declarations:
// Python definitions and classes, Go declarations, and anything else by
// falling back to token windows.
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

// Chunker splits source into declaration-sized windows. Text in a language
// it cannot cut falls back to token windows.
type Chunker struct {
	tok  tokenizer.Tokenizer
	size int
	lang string
	hard tokenchunker.Chunker
}

// New builds a Chunker. With no Option the language is detected: Go when the
// source parses as Go, Python when it reads as Python, and otherwise the
// token fallback. Use Language to skip the detection when the caller already
// knows, which is the case for a Markdown fence that names its language and
// for a CLI flag.
func New(tok tokenizer.Tokenizer, size int, opts ...Option) (Chunker, error) {
	if tok == nil {
		return Chunker{}, fmt.Errorf("tokenizer is required")
	}
	if size <= 0 {
		return Chunker{}, fmt.Errorf("size must be > 0, got %d", size)
	}
	c := Chunker{tok: tok, size: size}
	for _, opt := range opts {
		if err := opt(&c); err != nil {
			return Chunker{}, err
		}
	}
	hard, err := tokenchunker.New(tok, size, 0)
	if err != nil {
		return Chunker{}, err
	}
	c.hard = hard
	return c, nil
}

// Option configures a Chunker.
type Option func(*Chunker) error

// Language cuts the source with the rules for name, instead of detecting it.
// An empty name means no preference, which is the same as leaving the Option
// out, so a caller with an optional flag can pass it through unguarded. A
// name that is not recognized is an error: a language this package cannot
// cut is better reported than silently tokenized. A name is matched
// case-insensitively, and the usual aliases (`py`, `python3`, `golang`) are
// accepted.
func Language(name string) Option {
	return func(c *Chunker) error {
		// An empty name is "no preference", which is the same as leaving the
		// Option out. It has to be tested before the lookup, because an
		// unknown name also normalizes to the empty string and would
		// otherwise be accepted as if it had asked for detection.
		if strings.TrimSpace(name) == "" {
			c.lang = langNone
			return nil
		}
		switch lang := normalizeLang(name); lang {
		case langGo, langPython:
			c.lang = lang
			return nil
		}
		return fmt.Errorf("unknown language %q", name)
	}
}

const (
	langNone   = ""
	langGo     = "go"
	langPython = "python"
)

func normalizeLang(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "go", "golang":
		return langGo
	case "py", "python", "python3", "python2":
		return langPython
	}
	return ""
}

// Chunk splits source. Offsets are rune indexes into text.
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
	switch c.lang {
	case langGo:
		return goDeclPieces(text)
	case langPython:
		return pythonPieces(text)
	case langNone:
		// Detection, for a caller that did not name a language. Go is tried
		// first because it is the stricter grammar: a Go file has to start
		// with `package`, so ordinary prose and Python source do not reach
		// the parser, and a Python file never parses as Go. Getting this
		// wrong is not costly either way: a file cut by the wrong scanner
		// still reconstructs, and a file cut by neither falls back to token
		// windows.
		if pieces := goDeclPieces(text); len(pieces) > 0 {
			return pieces
		}
		return pythonPieces(text)
	}
	return nil
}

func goDeclPieces(text string) []piece {
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
