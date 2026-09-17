package codechunker

import (
	"testing"

	"github.com/bluesky585/nibble/internal/corpus"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

func BenchmarkChunkGo(b *testing.B) {
	text := corpus.GoSource()
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		c, err := New(tokenizer.Character{}, 512, Language("go"))
		if err != nil {
			b.Fatal(err)
		}
		if _, err := c.Chunk(text); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkChunkPython measures the line scanner, which is the slower of
// the two readers: Go is parsed by the standard library, Python is read
// by a hand-written bracket-depth scan.
func BenchmarkChunkPython(b *testing.B) {
	text := corpus.PythonSource()
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		c, err := New(tokenizer.Character{}, 512, Language("python"))
		if err != nil {
			b.Fatal(err)
		}
		if _, err := c.Chunk(text); err != nil {
			b.Fatal(err)
		}
	}
}
