package sentencechunker

import (
	"strings"
	"testing"

	"github.com/bluesky585/nibble/internal/assertchunk"
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
}

func TestChunkPacksSentences(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 20, nil)
	if err != nil {
		t.Fatal(err)
	}

	original := "Hello. How are you? Fine!"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2 (Hello. How are you? | Fine!)", len(got))
	}
	if got[0].Text != "Hello. How are you?" {
		t.Fatalf("first=%q", got[0].Text)
	}
	if got[1].Text != " Fine!" {
		t.Fatalf("second=%q", got[1].Text)
	}
}

func TestChunkOversizedSentence(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 4, nil)
	if err != nil {
		t.Fatal(err)
	}

	original := "Hello. Hi."
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	// "Hello." is 6 runes > 4, emitted whole; " Hi." is 4.
	assertchunk.Split(t, original, got)
	if len(got) != 2 || got[0].Text != "Hello." || got[1].Text != " Hi." {
		t.Fatalf("got %+v", got)
	}
}

func TestChunkCJK(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 8, nil)
	if err != nil {
		t.Fatal(err)
	}

	original := "你好。世界！"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if got[0].End != utf8Count(got[0].Text) {
		t.Fatalf("offsets should be runes: %+v", got[0])
	}
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

func TestChunkNoSentenceDelim(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 8, nil)
	if err != nil {
		t.Fatal(err)
	}
	original := "no punctuation here"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Text != original {
		t.Fatalf("got %+v", got)
	}
	assertchunk.Split(t, original, got)
}

func utf8Count(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}
