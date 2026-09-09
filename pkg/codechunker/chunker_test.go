package codechunker

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

func TestChunkPacksSmallFile(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 512)
	if err != nil {
		t.Fatal(err)
	}

	original := "package p\n\nfunc A() {}\n\nfunc B() {}\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) != 1 {
		t.Fatalf("len=%d want 1", len(got))
	}
}

func TestChunkSplitsFunctions(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 24)
	if err != nil {
		t.Fatal(err)
	}

	original := "package p\n\nfunc A() {}\n\nfunc B() {}\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) < 2 {
		t.Fatalf("expected funcs in separate chunks, got %+v", got)
	}
	joined := ""
	for _, ch := range got {
		joined += ch.Text
	}
	if !strings.Contains(joined, "func A()") || !strings.Contains(joined, "func B()") {
		t.Fatalf("missing funcs: %+v", got)
	}
}

func TestChunkKeepsDocCommentWithFunc(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 40)
	if err != nil {
		t.Fatal(err)
	}

	original := "package p\n\n// Hello does a thing.\nfunc Hello() {}\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)

	found := false
	for _, ch := range got {
		if strings.Contains(ch.Text, "Hello does a thing") && strings.Contains(ch.Text, "func Hello") {
			found = true
		}
	}
	if !found {
		t.Fatalf("doc comment should stay with the func: %+v", got)
	}
}

func TestChunkParseFallback(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 3)
	if err != nil {
		t.Fatal(err)
	}

	original := "not go source at all"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if got[0].Text != "not" {
		t.Fatalf("expected token fallback, got %+v", got)
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

func TestChunkUnicodeComment(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 512)
	if err != nil {
		t.Fatal(err)
	}

	original := "package p\n\n// 你好\nfunc A() {}\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if got[0].End != runeCount(original) {
		t.Fatalf("end=%d want %d", got[0].End, runeCount(original))
	}
}

func runeCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
