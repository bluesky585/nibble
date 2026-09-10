package overlap

import (
	"strings"
	"testing"

	"github.com/bluesky585/nibble/internal/assertchunk"
	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

func mustChunk(t *testing.T, text string, start int) chunk.Chunk {
	t.Helper()
	end := start
	for range text {
		end++
	}
	c, err := chunk.New(text, start, end, end-start)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPrefix(t *testing.T) {
	t.Parallel()

	original := "hello world"
	chunks := []chunk.Chunk{
		mustChunk(t, "hello ", 0),
		mustChunk(t, "world", 6),
	}

	got, err := Prefix(chunks, tokenizer.Character{}, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if got[0].Context != "" {
		t.Fatalf("first context=%q", got[0].Context)
	}
	if got[1].Context != "lo " {
		t.Fatalf("second context=%q want %q", got[1].Context, "lo ")
	}
	if got[1].Text != "world" {
		t.Fatalf("text must stay %q", got[1].Text)
	}
}

func TestPrefixKeepsExistingContext(t *testing.T) {
	t.Parallel()

	chunks := []chunk.Chunk{
		mustChunk(t, "ab", 0),
		mustChunk(t, "cd", 2),
	}
	chunks[1].Context = "H|"

	got, err := Prefix(chunks, tokenizer.Character{}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got[1].Context != "H|b" {
		t.Fatalf("context=%q", got[1].Context)
	}
}

func TestPrefixZero(t *testing.T) {
	t.Parallel()

	chunks := []chunk.Chunk{mustChunk(t, "ab", 0), mustChunk(t, "cd", 2)}
	got, err := Prefix(chunks, tokenizer.Character{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got[1].Context != "" {
		t.Fatalf("context=%q", got[1].Context)
	}
}

func TestPrefixShortPrevious(t *testing.T) {
	t.Parallel()

	chunks := []chunk.Chunk{mustChunk(t, "ab", 0), mustChunk(t, "cd", 2)}
	got, err := Prefix(chunks, tokenizer.Character{}, 8)
	if err != nil {
		t.Fatal(err)
	}
	if got[1].Context != "ab" {
		t.Fatalf("context=%q", got[1].Context)
	}
}

func TestPrefixValidation(t *testing.T) {
	t.Parallel()

	_, err := Prefix(nil, nil, 1)
	if err == nil || !strings.Contains(err.Error(), "tokenizer is required") {
		t.Fatalf("err=%v", err)
	}
	_, err = Prefix(nil, tokenizer.Character{}, -1)
	if err == nil || !strings.Contains(err.Error(), "n must be >= 0") {
		t.Fatalf("err=%v", err)
	}
}
