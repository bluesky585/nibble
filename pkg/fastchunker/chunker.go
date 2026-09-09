// Package fastchunker splits text by a byte budget, preferring delimiter
// boundaries. Chunk offsets are still rune indexes.
package fastchunker

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/bluesky585/nibble/pkg/chunk"
)

// DefaultDelimiters are tried from the end of a byte window. Longer
// matches that end later win.
var DefaultDelimiters = []string{
	"\n\n", "\r\n", "\n", "\r",
	"。", "！", "？",
	". ", "? ", "! ",
	".", "?", "!",
	" ", "\t",
}

// Chunker cuts text into windows of at most Size bytes, except a single
// rune that is longer than Size is kept whole so UTF-8 is never split.
type Chunker struct {
	size   int
	delims []string
}

// New builds a Chunker. Empty delims use DefaultDelimiters.
func New(size int, delims []string) (Chunker, error) {
	if size <= 0 {
		return Chunker{}, fmt.Errorf("size must be > 0, got %d", size)
	}
	for _, d := range delims {
		if d == "" {
			return Chunker{}, fmt.Errorf("delimiters must not be empty strings")
		}
	}
	if len(delims) == 0 {
		delims = append([]string(nil), DefaultDelimiters...)
	} else {
		delims = append([]string(nil), delims...)
	}
	return Chunker{size: size, delims: delims}, nil
}

// Chunk splits text. Size is a byte budget; Start/End are rune offsets.
func (c Chunker) Chunk(text string) ([]chunk.Chunk, error) {
	if text == "" {
		return nil, nil
	}

	var out []chunk.Chunk
	runeStart := 0
	for start := 0; start < len(text); {
		end := c.cutEnd(text, start)
		piece := text[start:end]
		n := utf8.RuneCountInString(piece)
		ch, err := chunk.New(piece, runeStart, runeStart+n, n)
		if err != nil {
			return nil, err
		}
		out = append(out, ch)
		runeStart += n
		start = end
	}
	return out, nil
}

func (c Chunker) cutEnd(text string, start int) int {
	if start+c.size >= len(text) {
		return len(text)
	}

	end := start + c.size
	for end > start && !utf8.RuneStart(text[end]) {
		end--
	}
	if end == start {
		_, w := utf8.DecodeRuneInString(text[start:])
		return start + w
	}

	window := text[start:end]
	best := -1
	for _, d := range c.delims {
		idx := strings.LastIndex(window, d)
		if idx < 0 {
			continue
		}
		cut := start + idx + len(d)
		if cut > start && cut <= end && cut > best {
			best = cut
		}
	}
	if best > start {
		return best
	}
	return end
}
