package store

import (
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/embed"
)

func mustChunk(t *testing.T, text string, start int) chunk.Chunk {
	t.Helper()
	n := utf8.RuneCountInString(text)
	c, err := chunk.New(text, start, start+n, n)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestMemorySearch(t *testing.T) {
	t.Parallel()

	chunks := []chunk.Chunk{
		mustChunk(t, "cats sleep on mats", 0),
		mustChunk(t, "quantum chromodynamics", 20),
	}
	mem := &Memory{}
	if err := Index(mem, embed.Hashing{}, chunks); err != nil {
		t.Fatal(err)
	}

	q, err := embed.Hashing{}.Embed([]string{"cats eat"})
	if err != nil {
		t.Fatal(err)
	}
	hits, err := mem.Search(q[0], 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("len=%d", len(hits))
	}
	if hits[0].Record.Chunk.Text != "cats sleep on mats" {
		t.Fatalf("top hit=%q", hits[0].Record.Chunk.Text)
	}
	if hits[0].Score < hits[1].Score {
		t.Fatalf("hits not ordered: %+v", hits)
	}
}

func TestJSONLRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "index.jsonl")
	st, err := OpenJSONL(path)
	if err != nil {
		t.Fatal(err)
	}

	chunks := []chunk.Chunk{
		mustChunk(t, "cats sleep", 0),
		mustChunk(t, "quantum field", 11),
	}
	if err := Index(st, embed.Hashing{}, chunks); err != nil {
		t.Fatal(err)
	}

	loaded, err := OpenJSONL(path)
	if err != nil {
		t.Fatal(err)
	}
	q, err := embed.Hashing{}.Embed([]string{"cats"})
	if err != nil {
		t.Fatal(err)
	}
	hits, err := loaded.Search(q[0], 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Record.Chunk.Text != "cats sleep" {
		t.Fatalf("got %+v", hits)
	}
}

func TestIndexEmpty(t *testing.T) {
	t.Parallel()

	mem := &Memory{}
	if err := Index(mem, embed.Hashing{}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestSearchBadK(t *testing.T) {
	t.Parallel()

	_, err := (&Memory{}).Search(nil, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}
