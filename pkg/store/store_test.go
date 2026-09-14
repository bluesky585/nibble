package store

import (
	"path/filepath"
	"reflect"
	"strings"
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

// Embed writes vectors onto the chunks without touching a store.
func TestEmbedFillsVectors(t *testing.T) {
	t.Parallel()

	chunks := []chunk.Chunk{
		mustChunk(t, "cats sleep", 0),
		mustChunk(t, "quantum field", 11),
	}
	got, err := Embed(embed.Hashing{}, chunks)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	for i, c := range got {
		if len(c.Embedding) == 0 {
			t.Fatalf("chunk %d has no embedding", i)
		}
		if c.Text != chunks[i].Text {
			t.Fatalf("chunk %d text changed: %q", i, c.Text)
		}
	}
	// The input slice is not modified in place.
	if len(chunks[0].Embedding) != 0 {
		t.Fatal("Embed must not mutate its input")
	}
}

// A chunk with Context embeds Context+Text, matching what Index stores.
func TestEmbedUsesContext(t *testing.T) {
	t.Parallel()

	c := mustChunk(t, "row", 0)
	c.Context = "| h |\n"
	got, err := Embed(embed.Hashing{}, []chunk.Chunk{c})
	if err != nil {
		t.Fatal(err)
	}

	want, err := embed.Hashing{}.Embed([]string{"| h |\nrow"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got[0].Embedding, want[0]) {
		t.Fatalf("vector does not match EmbedText")
	}
}

func TestEmbedErrors(t *testing.T) {
	t.Parallel()

	if _, err := Embed(nil, []chunk.Chunk{mustChunk(t, "a", 0)}); err == nil {
		t.Fatal("expected an error for a nil embedder")
	}
	got, err := Embed(embed.Hashing{}, nil)
	if err != nil || got != nil {
		t.Fatalf("empty input should pass through: %v %v", got, err)
	}
}

// IndexEmbedded reuses vectors already on the chunks instead of
// embedding again, and stores each vector once.
func TestIndexEmbedded(t *testing.T) {
	t.Parallel()

	chunks := []chunk.Chunk{
		mustChunk(t, "cats sleep", 0),
		mustChunk(t, "quantum field", 11),
	}
	embedded, err := Embed(embed.Hashing{}, chunks)
	if err != nil {
		t.Fatal(err)
	}

	mem := &Memory{}
	if err := IndexEmbedded(mem, embedded); err != nil {
		t.Fatal(err)
	}
	q, err := embed.Hashing{}.Embed([]string{"cats"})
	if err != nil {
		t.Fatal(err)
	}
	hits, err := mem.Search(q[0], 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Record.Chunk.Text != "cats sleep" {
		t.Fatalf("got %+v", hits)
	}
	// The stored chunk copy must not carry a duplicate vector.
	if len(hits[0].Record.Chunk.Embedding) != 0 {
		t.Fatal("stored chunk should not repeat its vector")
	}
}

func TestIndexEmbeddedErrors(t *testing.T) {
	t.Parallel()

	if err := IndexEmbedded(nil, nil); err == nil {
		t.Fatal("expected an error for a nil store")
	}
	// A chunk without a vector is a caller bug, not a silent skip.
	err := IndexEmbedded(&Memory{}, []chunk.Chunk{mustChunk(t, "a", 0)})
	if err == nil || !strings.Contains(err.Error(), "has no embedding") {
		t.Fatalf("err=%v", err)
	}
}

// countingEmbedder records how many batch calls it received and how many
// texts each carried, so a test can assert the client is used in batches
// rather than once per chunk.
type countingEmbedder struct {
	calls int
	sizes []int
}

func (e *countingEmbedder) Embed(texts []string) ([][]float64, error) {
	e.calls++
	e.sizes = append(e.sizes, len(texts))
	out := make([][]float64, len(texts))
	for i := range texts {
		out[i] = []float64{float64(i)}
	}
	return out, nil
}

// The whole point of the batch API: many chunks must cost one call.
func TestEmbedAndIndexEmbeddedUseOneCall(t *testing.T) {
	t.Parallel()

	chunks := make([]chunk.Chunk, 5)
	for i := range chunks {
		chunks[i] = mustChunk(t, "text", i)
	}

	counter := &countingEmbedder{}
	embedded, err := Embed(counter, chunks)
	if err != nil {
		t.Fatal(err)
	}
	if counter.calls != 1 || counter.sizes[0] != 5 {
		t.Fatalf("Embed should batch: calls=%d sizes=%v", counter.calls, counter.sizes)
	}

	mem := &Memory{}
	if err := IndexEmbedded(mem, embedded); err != nil {
		t.Fatal(err)
	}
	// IndexEmbedded must not embed at all; it only reuses vectors.
	if counter.calls != 1 {
		t.Fatalf("IndexEmbedded made a call: calls=%d", counter.calls)
	}
}

// A mismatched vector count is an error, not a silent short index.
func TestEmbedCountMismatch(t *testing.T) {
	t.Parallel()

	if _, err := Embed(shortEmbedder{}, []chunk.Chunk{mustChunk(t, "a", 0), mustChunk(t, "b", 1)}); err == nil {
		t.Fatal("expected an error for a vector count mismatch")
	}
}

type shortEmbedder struct{}

func (shortEmbedder) Embed(texts []string) ([][]float64, error) {
	return [][]float64{{0}}, nil
}

// A query from a different embedder has the wrong width. Cosine would
// score every record 0 and return a meaningless ranking, so it is an
// error instead.
func TestSearchDimensionMismatch(t *testing.T) {
	t.Parallel()

	mem := &Memory{}
	if err := Index(mem, embed.Hashing{}, []chunk.Chunk{mustChunk(t, "cats sleep", 0)}); err != nil {
		t.Fatal(err)
	}
	_, err := mem.Search([]float64{1, 2, 3}, 1)
	if err == nil || !strings.Contains(err.Error(), "dimensions") {
		t.Fatalf("err=%v", err)
	}
}

// An empty store has nothing to compare against, so any query is fine.
func TestSearchEmptyStore(t *testing.T) {
	t.Parallel()

	hits, err := (&Memory{}).Search([]float64{1, 2, 3}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("got %+v", hits)
	}
}

func TestSearchBadK(t *testing.T) {
	t.Parallel()

	_, err := (&Memory{}).Search(nil, 0)
	if err == nil {
		t.Fatal("expected error")
	}
}
