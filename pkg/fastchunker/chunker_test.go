package fastchunker

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/bluesky585/nibble/internal/assertchunk"
)

func TestNewValidation(t *testing.T) {
	t.Parallel()

	_, err := New(0, nil)
	if err == nil || !strings.Contains(err.Error(), "size must be > 0") {
		t.Fatalf("err=%v", err)
	}
	_, err = New(8, []string{""})
	if err == nil || !strings.Contains(err.Error(), "delimiters must not be empty") {
		t.Fatalf("err=%v", err)
	}
}

func TestChunkPrefersDelimiter(t *testing.T) {
	t.Parallel()

	c, err := New(8, []string{" "})
	if err != nil {
		t.Fatal(err)
	}

	original := "hello world"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) != 2 || got[0].Text != "hello " || got[1].Text != "world" {
		t.Fatalf("got %+v", got)
	}
}

func TestChunkHardCutNoDelim(t *testing.T) {
	t.Parallel()

	c, err := New(3, []string{" "})
	if err != nil {
		t.Fatal(err)
	}

	original := "abcdefgh"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if got[0].Text != "abc" || got[1].Text != "def" || got[2].Text != "gh" {
		t.Fatalf("got %+v", got)
	}
}

func TestChunkDoesNotSplitRune(t *testing.T) {
	t.Parallel()

	c, err := New(1, nil)
	if err != nil {
		t.Fatal(err)
	}

	original := "你好"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) != 2 || got[0].Text != "你" || got[1].Text != "好" {
		t.Fatalf("got %+v", got)
	}
	if got[0].Start != 0 || got[0].End != 1 || got[1].End != 2 {
		t.Fatalf("rune offsets %+v %+v (not bytes; 你 is 3 bytes)", got[0], got[1])
	}
	if utf8.RuneCountInString(original) != 2 || len(original) != 6 {
		t.Fatal("precondition")
	}
}

func TestChunkNewline(t *testing.T) {
	t.Parallel()

	c, err := New(20, nil)
	if err != nil {
		t.Fatal(err)
	}

	original := "first line\n\nsecond"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
}

func TestChunkEmpty(t *testing.T) {
	t.Parallel()

	c, err := New(8, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Chunk("")
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, "", got)
}

func TestChunkShorterThanSize(t *testing.T) {
	t.Parallel()

	c, err := New(64, nil)
	if err != nil {
		t.Fatal(err)
	}
	original := "hi"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Text != original {
		t.Fatalf("got %+v", got)
	}
	assertchunk.Split(t, original, got)
}
