// The store round trip: chunk, embed, index to a JSONL file, reopen
// the file in a fresh store, and search it. This is the whole index
// lifecycle in one process — the same file works across processes, and
// the CLI reads and writes the same shape.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/embed"
	"github.com/bluesky585/nibble/pkg/store"
)

func main() {
	dir, err := os.MkdirTemp("", "nibble-example")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "index.jsonl")

	// Build: chunk some texts and index them with the local embedder.
	st, err := store.OpenJSONL(path)
	if err != nil {
		panic(err)
	}
	emb := embed.Hashing{}
	texts := []string{
		"Cats sleep most of the day.",
		"Dogs bark at strangers.",
		"Birds migrate in autumn.",
	}
	var chunks []chunk.Chunk
	for i, text := range texts {
		ch, err := chunk.New(text, i*40, i*40+len([]rune(text)), len([]rune(text)))
		if err != nil {
			panic(err)
		}
		chunks = append(chunks, ch)
	}
	if err := store.Index(st, emb, chunks); err != nil {
		panic(err)
	}

	// Reopen: the file is the store, so a fresh handle finds everything.
	st2, err := store.OpenJSONL(path)
	if err != nil {
		panic(err)
	}
	vecs, err := emb.Embed([]string{"what do cats do"})
	if err != nil {
		panic(err)
	}
	hits, err := st2.Search(vecs[0], 2)
	if err != nil {
		panic(err)
	}
	for _, h := range hits {
		fmt.Printf("%.3f %q\n", h.Score, h.Record.Chunk.Text)
	}
}
