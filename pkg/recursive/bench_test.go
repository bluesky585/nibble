package recursive

import (
	"testing"

	"github.com/bluesky585/nibble/internal/corpus"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

func BenchmarkChunk(b *testing.B) {
	text := corpus.Prose()
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		c, err := New(tokenizer.Character{}, 512, nil)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := c.Chunk(text); err != nil {
			b.Fatal(err)
		}
	}
}
