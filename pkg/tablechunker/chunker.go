// Package tablechunker splits Markdown tables by row while keeping the
// header available on later chunks as Context.
package tablechunker

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/tokenchunker"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

type line struct {
	Text  string
	Start int
	End   int
}

// Chunker splits Markdown tables by rows. Non-table text is packed with
// the token chunker when it exceeds Size.
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

// Chunk splits text. Table continuation chunks copy the header into Context.
func (c Chunker) Chunk(text string) ([]chunk.Chunk, error) {
	lines := splitLines(text)
	if len(lines) == 0 {
		return nil, nil
	}

	var out []chunk.Chunk
	for i := 0; i < len(lines); {
		if isTableStart(lines, i) {
			end := tableEnd(lines, i)
			chunks, err := c.chunkTable(lines[i:end])
			if err != nil {
				return nil, err
			}
			out = append(out, chunks...)
			i = end
			continue
		}
		end := i + 1
		for end < len(lines) && !isTableStart(lines, end) {
			end++
		}
		chunks, err := c.chunkProse(lines[i:end])
		if err != nil {
			return nil, err
		}
		out = append(out, chunks...)
		i = end
	}
	return out, nil
}

func (c Chunker) chunkProse(lines []line) ([]chunk.Chunk, error) {
	text := join(lines)
	n := c.tok.Count(text)
	if n <= c.size {
		ch, err := chunk.New(text, lines[0].Start, lines[len(lines)-1].End, n)
		if err != nil {
			return nil, err
		}
		return []chunk.Chunk{ch}, nil
	}
	chunks, err := c.hard.Chunk(text)
	if err != nil {
		return nil, err
	}
	offset := lines[0].Start
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

func (c Chunker) chunkTable(lines []line) ([]chunk.Chunk, error) {
	header := []line{lines[0], lines[1]}
	headerText := join(header)
	rows := lines[2:]

	var out []chunk.Chunk
	flush := func(group []line, withHeader bool) error {
		if len(group) == 0 {
			return nil
		}
		text := join(group)
		ch, err := chunk.New(text, group[0].Start, group[len(group)-1].End, c.tok.Count(text))
		if err != nil {
			return err
		}
		if !withHeader {
			ch.Context = headerText
		}
		out = append(out, ch)
		return nil
	}

	if len(rows) == 0 {
		if err := flush(header, true); err != nil {
			return nil, err
		}
		return out, nil
	}

	group := append([]line(nil), header...)
	tokens := c.tok.Count(headerText)
	first := true
	for _, row := range rows {
		n := c.tok.Count(row.Text)
		if len(group) > 0 && tokens+n > c.size && (!first || len(group) > len(header)) {
			if err := flush(group, first); err != nil {
				return nil, err
			}
			group = nil
			tokens = 0
			first = false
		}
		if first && len(group) == len(header) && tokens+n > c.size {
			if err := flush(group, true); err != nil {
				return nil, err
			}
			group = []line{row}
			tokens = n
			first = false
			continue
		}
		group = append(group, row)
		tokens += n
	}
	if err := flush(group, first); err != nil {
		return nil, err
	}
	return out, nil
}

func splitLines(text string) []line {
	if text == "" {
		return nil
	}
	var out []line
	startB, startR := 0, 0
	b, r := 0, 0
	for b < len(text) {
		if text[b] == '\n' {
			endB := b + 1
			endR := r + 1
			out = append(out, line{Text: text[startB:endB], Start: startR, End: endR})
			startB, startR = endB, endR
			b, r = endB, endR
			continue
		}
		_, w := utf8.DecodeRuneInString(text[b:])
		b += w
		r++
	}
	if startB < len(text) {
		out = append(out, line{Text: text[startB:], Start: startR, End: r})
	}
	return out
}

func join(lines []line) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.Text)
	}
	return b.String()
}

func isTableStart(lines []line, i int) bool {
	return i+1 < len(lines) && isPipeRow(lines[i].Text) && isSeparator(lines[i+1].Text)
}

func tableEnd(lines []line, i int) int {
	j := i + 2
	for j < len(lines) && isPipeRow(lines[j].Text) {
		j++
	}
	return j
}

func isPipeRow(s string) bool {
	t := strings.TrimSpace(s)
	return strings.HasPrefix(t, "|") && strings.Count(t, "|") >= 2
}

func isSeparator(s string) bool {
	t := strings.TrimSpace(s)
	if !strings.Contains(t, "|") || !strings.Contains(t, "-") {
		return false
	}
	for _, r := range t {
		switch r {
		case '|', '-', ':', ' ', '\t', '\n', '\r':
		default:
			return false
		}
	}
	return true
}
