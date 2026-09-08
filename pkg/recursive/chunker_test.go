package recursive

import (
	"strings"
	"testing"

	"github.com/bluesky585/nibble/internal/assertchunk"
	"github.com/bluesky585/nibble/pkg/split"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

func TestNewValidation(t *testing.T) {
	t.Parallel()

	_, err := New(nil, 8, nil)
	if err == nil || !strings.Contains(err.Error(), "tokenizer is required") {
		t.Fatalf("err=%v", err)
	}
	_, err = New(tokenizer.Character{}, 0, nil)
	if err == nil || !strings.Contains(err.Error(), "size must be > 0") {
		t.Fatalf("err=%v", err)
	}
	_, err = New(tokenizer.Character{}, 8, []Level{
		{Token: true, Delimiters: []string{"."}},
	})
	if err == nil || !strings.Contains(err.Error(), "token level cannot set delimiters") {
		t.Fatalf("err=%v", err)
	}
}

func TestChunkShortText(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 64, nil)
	if err != nil {
		t.Fatal(err)
	}
	original := "hello"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Text != original {
		t.Fatalf("got %+v", got)
	}
	assertchunk.Split(t, original, got)
}

func TestChunkParagraphThenSentence(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 12, nil)
	if err != nil {
		t.Fatal(err)
	}

	original := "Hello. World.\n\nHi. There."
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)

	// "Hello. World.\n\n" is 15 runes > 12, so it drops to sentences.
	if len(got) < 2 {
		t.Fatalf("expected recursion into sentences, got %+v", got)
	}
	if got[0].Text != "Hello." {
		t.Fatalf("first=%q", got[0].Text)
	}
}

func TestChunkTokenFallback(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 3, []Level{{Token: true}})
	if err != nil {
		t.Fatal(err)
	}

	original := "abcdefgh"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	if got[0].Text != "abc" || got[1].Text != "def" || got[2].Text != "gh" {
		t.Fatalf("got %q %q %q", got[0].Text, got[1].Text, got[2].Text)
	}
}

func TestChunkUnicode(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 3, nil)
	if err != nil {
		t.Fatal(err)
	}

	original := "你好。世界！"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if got[0].Text != "你好。" || got[1].Text != "世界！" {
		t.Fatalf("got %+v", got)
	}
	if got[0].End != 3 || got[1].Start != 3 {
		t.Fatalf("rune offsets %+v %+v", got[0], got[1])
	}
}

func TestChunkSingleLevelDelimiters(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 4, []Level{
		{Delimiters: []string{"."}, Attach: split.AttachPrev},
		{Token: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	original := "aa.bb.cc"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
}

func TestChunkEmpty(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 8, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Chunk("")
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, "", got)
}

func TestChunkNoDelimiterFallsThrough(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 3, []Level{
		{Delimiters: []string{"."}, Attach: split.AttachPrev},
		{Token: true},
	})
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
		t.Fatalf("expected token fallback, got %+v", got)
	}
}
