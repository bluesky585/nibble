package tokenchunker

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/bluesky585/nibble/internal/assertchunk"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

func TestNewValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tok     tokenizer.Tokenizer
		size    int
		overlap int
		wantErr string
	}{
		{name: "ok", tok: tokenizer.Character{}, size: 8, overlap: 0},
		{name: "nil tokenizer", tok: nil, size: 8, overlap: 0, wantErr: "tokenizer is required"},
		{name: "size zero", tok: tokenizer.Character{}, size: 0, overlap: 0, wantErr: "size must be > 0"},
		{name: "negative overlap", tok: tokenizer.Character{}, size: 8, overlap: -1, wantErr: "overlap must be >= 0"},
		{name: "overlap equals size", tok: tokenizer.Character{}, size: 4, overlap: 4, wantErr: "overlap must be < size"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := New(tt.tok, tt.size, tt.overlap)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestChunkCharacterNoOverlap(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 2, 0)
	if err != nil {
		t.Fatal(err)
	}

	original := "hello"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	if got[0].Text != "he" || got[1].Text != "ll" || got[2].Text != "o" {
		t.Fatalf("texts=%q %q %q", got[0].Text, got[1].Text, got[2].Text)
	}
	assertchunk.Split(t, original, got)
}

func TestChunkUnicode(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 1, 0)
	if err != nil {
		t.Fatal(err)
	}

	original := "你好"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Text != "你" || got[1].Text != "好" {
		t.Fatalf("got %+v", got)
	}
	if got[0].End != 1 || got[1].Start != 1 || got[1].End != 2 {
		t.Fatalf("offsets %+v %+v (2 runes, not 6 bytes)", got[0], got[1])
	}
	assertchunk.Split(t, original, got)
}

func TestChunkEmpty(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 8, 0)
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

	c, err := New(tokenizer.Character{}, 64, 0)
	if err != nil {
		t.Fatal(err)
	}
	original := "hi"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Text != original || got[0].TokenCount != 2 {
		t.Fatalf("got %+v", got)
	}
	assertchunk.Split(t, original, got)
}

func TestChunkWord(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Word{}, 3, 0)
	if err != nil {
		t.Fatal(err)
	}
	// "hello world" -> ["hello", " ", "world"] : one chunk of 3 tokens
	original := "hello world"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Text != original {
		t.Fatalf("got %+v", got)
	}
	assertchunk.Split(t, original, got)

	original = "one two three four"
	got, err = c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
}

func TestChunkOverlap(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 3, 1)
	if err != nil {
		t.Fatal(err)
	}

	original := "hello"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	if got[0].Text != "hel" || got[1].Text != "llo" {
		t.Fatalf("texts=%q %q", got[0].Text, got[1].Text)
	}
	if got[1].Start != 2 {
		t.Fatalf("overlap start=%d want 2", got[1].Start)
	}

	runes := []rune(original)
	if got[0].Start != 0 || got[len(got)-1].End != len(runes) {
		t.Fatalf("does not cover original: %+v", got)
	}
	for i, ch := range got {
		slice := string(runes[ch.Start:ch.End])
		if slice != ch.Text {
			t.Fatalf("chunk %d text %q != original[%d:%d] %q", i, ch.Text, ch.Start, ch.End, slice)
		}
	}
	if err := assertchunk.Check(original, got); err == nil {
		t.Fatal("overlap split should fail the no-gap reconstruct check")
	}
}

func TestChunkTokenCount(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Chunk("你好世界")
	if err != nil {
		t.Fatal(err)
	}
	if utf8.RuneCountInString("你好世界") != 4 {
		t.Fatal("precondition")
	}
	if got[0].TokenCount != 2 || got[1].TokenCount != 2 {
		t.Fatalf("token counts %+v %+v", got[0], got[1])
	}
}
