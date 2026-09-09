package tablechunker

import (
	"strings"
	"testing"

	"github.com/bluesky585/nibble/internal/assertchunk"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

func TestNewValidation(t *testing.T) {
	t.Parallel()

	_, err := New(nil, 8)
	if err == nil || !strings.Contains(err.Error(), "tokenizer is required") {
		t.Fatalf("err=%v", err)
	}
	_, err = New(tokenizer.Character{}, 0)
	if err == nil || !strings.Contains(err.Error(), "size must be > 0") {
		t.Fatalf("err=%v", err)
	}
}

func TestChunkSmallTable(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 64)
	if err != nil {
		t.Fatal(err)
	}

	original := "| h |\n| --- |\n| 1 |\n| 2 |\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) != 1 {
		t.Fatalf("len=%d want 1", len(got))
	}
	if got[0].Context != "" {
		t.Fatalf("single chunk should not copy header: %q", got[0].Context)
	}
}

func TestChunkRepeatsHeaderInContext(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 18)
	if err != nil {
		t.Fatal(err)
	}

	original := "| h |\n| --- |\n| 1 |\n| 2 |\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) < 2 {
		t.Fatalf("expected a split table, got %+v", got)
	}
	if !strings.HasPrefix(got[0].Text, "| h |") {
		t.Fatalf("first=%q", got[0].Text)
	}
	if got[len(got)-1].Context == "" {
		t.Fatal("continuation chunk needs header context")
	}
	if !strings.Contains(got[len(got)-1].Context, "| h |") {
		t.Fatalf("context=%q", got[len(got)-1].Context)
	}
	if strings.Contains(got[len(got)-1].Text, "| h |") {
		t.Fatalf("continuation text should not repeat header: %q", got[len(got)-1].Text)
	}
}

func TestChunkProseAndTable(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 64)
	if err != nil {
		t.Fatal(err)
	}

	original := "intro\n\n| h |\n| --- |\n| 1 |\n\noutro\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) != 3 {
		t.Fatalf("len=%d want 3 (prose, table, prose), got %+v", len(got), got)
	}
}

func TestChunkUnicode(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 64)
	if err != nil {
		t.Fatal(err)
	}

	original := "| 列 |\n| --- |\n| 值 |\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if got[0].End != runeCount(original) {
		t.Fatalf("end=%d want %d", got[0].End, runeCount(original))
	}
}

func TestChunkEmpty(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 8)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Chunk("")
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, "", got)
}

func TestChunkOversizedProse(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 3)
	if err != nil {
		t.Fatal(err)
	}
	original := "abcdefgh"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if got[0].Text != "abc" {
		t.Fatalf("got %+v", got)
	}
}

func runeCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
