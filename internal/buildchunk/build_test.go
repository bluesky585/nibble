package buildchunk

import (
	"strings"
	"testing"

	"github.com/bluesky585/nibble/internal/assertchunk"
)

func TestNewRecursive(t *testing.T) {
	t.Parallel()

	c, err := New("recursive", "character", "", 64, 0, nil)
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

	_, err := New("sentence", "character", "", 64, 1, nil)
	if err == nil || !strings.Contains(err.Error(), "overlap is only supported by the token chunker") {
		t.Fatalf("err=%v", err)
	}
}

func TestNewFast(t *testing.T) {
	t.Parallel()

	c, err := New("fast", "character", "", 8, 0, nil)
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

	c, err := New("markdown", "character", "", 24, 0, nil)
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

	_, err := New("markdown", "character", "", 24, 1, nil)
	if err == nil || !strings.Contains(err.Error(), "overlap is only supported by the token chunker") {
		t.Fatalf("err=%v", err)
	}
}

func TestNewUnknown(t *testing.T) {
	t.Parallel()

	_, err := New("magic", "character", "", 8, 0, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown chunker") {
		t.Fatalf("err=%v", err)
	}
	_, err = New("token", "emoji", "", 8, 0, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown tokenizer") {
		t.Fatalf("err=%v", err)
	}
}

// -lang reaches the code chunker: naming Python must cut by Python rules,
// not by whatever the source happens to look like. The input is not valid Go
// and has two top-level defs.
//
// The budget is deliberately too small to hold both defs, because the code
// chunker packs pieces that fit back together: at a size that holds the whole
// input it would answer with one chunk no matter where it cut.
func TestNewCodeLang(t *testing.T) {
	t.Parallel()

	c, err := New("code", "character", "python", 24, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	original := "def a():\n    pass\n\n\ndef b():\n    pass\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2: %+v", len(got), got)
	}
	if got[1].Text != "def b():\n    pass\n" {
		t.Fatalf("second chunk=%q, want the boundary at the second def", got[1].Text)
	}
}

// A language the code chunker cannot cut is an error, not a silent fallback:
// -lang naming a language that was ignored would look like it worked.
func TestNewCodeLangUnknown(t *testing.T) {
	t.Parallel()

	_, err := New("code", "character", "rust", 512, 0, nil)
	if err == nil || !strings.Contains(err.Error(), `unknown language "rust"`) {
		t.Fatalf("err=%v", err)
	}
}

// -lang is meaningless for markdown: each fence names its own language, so a
// document-wide flag would have to be ignored or override the fence.
func TestNewMarkdownRejectsLang(t *testing.T) {
	t.Parallel()

	_, err := New("markdown", "character", "python", 24, 0, nil)
	if err == nil || !strings.Contains(err.Error(), "-lang is only used with -chunker code") {
		t.Fatalf("err=%v", err)
	}
}
