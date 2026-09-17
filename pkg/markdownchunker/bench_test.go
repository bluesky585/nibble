package markdownchunker

import (
	"testing"

	"github.com/bluesky585/nibble/internal/corpus"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

// BenchmarkChunk measures the mixed Markdown document: prose routes to
// recursive, the table to the table chunker, and the fenced Python block
// to the code chunker, so this one benchmark covers the routing cost and
// all three paths it dispatches to.
func BenchmarkChunk(b *testing.B) {
	text := corpus.Mixed()
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		c, err := New(tokenizer.Character{}, 512)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := c.Chunk(text); err != nil {
			b.Fatal(err)
		}
	}
}
