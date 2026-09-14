package buildchunk

import (
	"strings"
	"testing"

	"github.com/bluesky585/nibble/internal/assertchunk"
)

func TestNewRecursive(t *testing.T) {
	t.Parallel()

	c, err := New("recursive", "character", 64, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	original := "Hello. World."
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
}

func TestNewOverlapRejected(t *testing.T) {
	t.Parallel()

	_, err := New("sentence", "character", 64, 1, nil)
	if err == nil || !strings.Contains(err.Error(), "overlap is only supported by the token chunker") {
		t.Fatalf("err=%v", err)
	}
}

func TestNewFast(t *testing.T) {
	t.Parallel()

	c, err := New("fast", "character", 8, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	original := "hello world"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
}

func TestNewMarkdown(t *testing.T) {
	t.Parallel()

	c, err := New("markdown", "character", 24, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	original := "intro\n\n| h |\n| --- |\n| 1 |\n\n```go\nfunc A() {}\n```\n\noutro\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
}

func TestNewMarkdownRejectsOverlap(t *testing.T) {
	t.Parallel()

	_, err := New("markdown", "character", 24, 1, nil)
	if err == nil || !strings.Contains(err.Error(), "overlap is only supported by the token chunker") {
		t.Fatalf("err=%v", err)
	}
}

func TestNewUnknown(t *testing.T) {
	t.Parallel()

	_, err := New("magic", "character", 8, 0, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown chunker") {
		t.Fatalf("err=%v", err)
	}
	_, err = New("token", "emoji", 8, 0, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown tokenizer") {
		t.Fatalf("err=%v", err)
	}
}
