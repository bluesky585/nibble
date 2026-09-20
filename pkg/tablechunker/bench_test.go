package tablechunker

import (
	"testing"

	"github.com/bluesky585/nibble/internal/corpus"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

func BenchmarkChunk(b *testing.B) {
	text := corpus.Mixed()
	// The chunker is built once outside the loop: construction is not
	// what this benchmark measures, and a caller reuses one chunker for
	// a whole run.
	c, err := New(tokenizer.Character{}, 512)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(text)))
	b.ResetTimer()
	for b.Loop() {
		if _, err := c.Chunk(text); err != nil {
			b.Fatal(err)
		}
	}
}
