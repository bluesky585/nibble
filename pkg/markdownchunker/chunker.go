// Package markdownchunker routes each part of a Markdown document to the
// chunker that understands it: fenced code blocks to the code chunker,
// GFM tables to the table chunker, and everything else to the recursive
// chunker. Regions are disjoint and cover the whole input, so chunk
// order and offsets stay reconstructable.
package markdownchunker

import (
	"fmt"
	"sort"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/codechunker"
	"github.com/bluesky585/nibble/pkg/markdown"
	"github.com/bluesky585/nibble/pkg/recursive"
	"github.com/bluesky585/nibble/pkg/tablechunker"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

type regionKind int

const (
	kindProse regionKind = iota
	kindCode
	kindTable
)

type region struct {
	kind  regionKind
	text  string
	start int
	end   int
	code  markdown.Code
}

// Chunker splits Markdown by region. Each region is delegated to a
// chunker for that content type, so a table is never cut mid-row and a
// code block is never parsed as prose.
//
// A fenced block is passed to the code chunker with its fence lines
// removed, because the fence syntax is not part of the language inside.
// The fence lines are then reattached to the boundary chunks, which
// keeps offsets contiguous. That can push a boundary chunk slightly over
// Size; interior code chunks still respect it.
type Chunker struct {
	tok   tokenizer.Tokenizer
	size  int
	prose recursive.Chunker
	code  codechunker.Chunker
	table tablechunker.Chunker
}

// New builds a Chunker.
func New(tok tokenizer.Tokenizer, size int) (Chunker, error) {
	if tok == nil {
		return Chunker{}, fmt.Errorf("tokenizer is required")
	}
	if size <= 0 {
		return Chunker{}, fmt.Errorf("size must be > 0, got %d", size)
	}
	prose, err := recursive.New(tok, size, nil)
	if err != nil {
		return Chunker{}, err
	}
	code, err := codechunker.New(tok, size)
	if err != nil {
		return Chunker{}, err
	}
	table, err := tablechunker.New(tok, size)
	if err != nil {
		return Chunker{}, err
	}
	return Chunker{tok: tok, size: size, prose: prose, code: code, table: table}, nil
}

// Chunk splits text. Offsets are rune indexes into text.
func (c Chunker) Chunk(text string) ([]chunk.Chunk, error) {
	if text == "" {
		return nil, nil
	}
	var out []chunk.Chunk
	for _, r := range regions([]rune(text), text) {
		var (
			part []chunk.Chunk
			err  error
		)
		switch r.kind {
		case kindCode:
			part, err = c.chunkCode(r)
		case kindTable:
			part, err = c.table.Chunk(r.text)
		default:
			part, err = c.prose.Chunk(r.text)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, shift(part, r.start)...)
	}
	return out, nil
}

// chunkCode feeds the code between the fence lines to the code chunker,
// then folds the fence lines back onto the first and last chunk. Offsets
// are relative to the region; the caller shifts them into the document.
func (c Chunker) chunkCode(r region) ([]chunk.Chunk, error) {
	runes := []rune(r.text)
	bodyStart := r.code.InnerStart - r.start
	bodyEnd := r.code.InnerEnd - r.start
	prefix, body, suffix := runes[:bodyStart], string(runes[bodyStart:bodyEnd]), runes[bodyEnd:]

	bodyChunks, err := c.code.Chunk(body)
	if err != nil {
		return nil, err
	}
	if len(bodyChunks) == 0 {
		// Empty body: the block is just its two fence lines.
		ch, err := chunk.New(r.text, 0, len(runes), c.tok.Count(r.text))
		if err != nil {
			return nil, err
		}
		return []chunk.Chunk{ch}, nil
	}

	out := make([]chunk.Chunk, 0, len(bodyChunks))
	for i, bc := range bodyChunks {
		part := []rune(bc.Text)
		start := bodyStart + bc.Start
		end := bodyStart + bc.End
		if i == 0 {
			part = append(append([]rune(nil), prefix...), part...)
			start = 0
		}
		if i == len(bodyChunks)-1 {
			part = append(part, suffix...)
			end = len(runes)
		}
		text := string(part)
		ch, err := chunk.New(text, start, end, c.tok.Count(text))
		if err != nil {
			return nil, err
		}
		out = append(out, ch)
	}
	return out, nil
}

type mark struct {
	start int
	end   int
	kind  regionKind
	code  markdown.Code
}

// regions cuts text into disjoint prose, code, and table spans in order.
//
// Code blocks are taken first. A table that intersects a code span is
// dropped: fences and pipe rows are both line-oriented, and a row inside
// a fence is code, not a table.
func regions(runes []rune, text string) []region {
	codes := markdown.CodeBlocks(text)
	var marks []mark
	for _, c := range codes {
		marks = append(marks, mark{start: c.Start, end: c.End, kind: kindCode, code: c})
	}
	for _, tb := range markdown.Tables(text) {
		if intersectsAny(codes, tb.Start, tb.End) {
			continue
		}
		marks = append(marks, mark{start: tb.Start, end: tb.End, kind: kindTable})
	}
	sort.SliceStable(marks, func(i, j int) bool { return marks[i].start < marks[j].start })

	var out []region
	prev := 0
	for _, m := range marks {
		if m.start > prev {
			out = append(out, region{
				kind:  kindProse,
				text:  string(runes[prev:m.start]),
				start: prev,
				end:   m.start,
			})
		}
		out = append(out, region{
			kind:  m.kind,
			text:  string(runes[m.start:m.end]),
			start: m.start,
			end:   m.end,
			code:  m.code,
		})
		prev = m.end
	}
	if prev < len(runes) {
		out = append(out, region{
			kind:  kindProse,
			text:  string(runes[prev:]),
			start: prev,
			end:   len(runes),
		})
	}
	return out
}

func intersectsAny(codes []markdown.Code, start, end int) bool {
	for _, c := range codes {
		if start < c.End && c.Start < end {
			return true
		}
	}
	return false
}

func shift(chunks []chunk.Chunk, offset int) []chunk.Chunk {
	if offset == 0 {
		return chunks
	}
	for i := range chunks {
		chunks[i].Start += offset
		chunks[i].End += offset
	}
	return chunks
}
