package semantic

import (
	"strings"
	"testing"

	"github.com/bluesky585/nibble/internal/assertchunk"
	"github.com/bluesky585/nibble/pkg/embed"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

func TestNewValidation(t *testing.T) {
	t.Parallel()

	_, err := New(nil, embed.Hashing{}, 8, 0)
	if err == nil || !strings.Contains(err.Error(), "tokenizer is required") {
		t.Fatalf("err=%v", err)
	}
	_, err = New(tokenizer.Character{}, nil, 8, 0)
	if err == nil || !strings.Contains(err.Error(), "embedder is required") {
		t.Fatalf("err=%v", err)
	}
	_, err = New(tokenizer.Character{}, embed.Hashing{}, 0, 0)
	if err == nil || !strings.Contains(err.Error(), "size must be > 0") {
		t.Fatalf("err=%v", err)
	}
	_, err = New(tokenizer.Character{}, embed.Hashing{}, 8, 2)
	if err == nil || !strings.Contains(err.Error(), "minSim") {
		t.Fatalf("err=%v", err)
	}
}

func TestChunkSplitsDissimilarSentences(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, embed.Hashing{}, 512, 0.5)
	if err != nil {
		t.Fatal(err)
	}

	original := "Cats sleep. Cats eat. Quantum chromodynamics is hard."
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) < 2 {
		t.Fatalf("expected a topic break, got %+v", got)
	}
	last := got[len(got)-1].Text
	if !strings.Contains(last, "Quantum") {
		t.Fatalf("quantum sentence should start a new chunk, got %+v", got)
	}
}

func TestChunkRespectsSize(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, embed.Hashing{}, 12, 0)
	if err != nil {
		t.Fatal(err)
	}

	original := "Cats sleep. Cats eat."
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) < 2 {
		t.Fatalf("size 12 should split these sentences, got %+v", got)
	}
}

func TestChunkEmpty(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, embed.Hashing{}, 8, 0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Chunk("")
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, "", got)
}

func TestChunkUnicode(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, embed.Hashing{}, 64, 0)
	if err != nil {
		t.Fatal(err)
	}

	original := "你好。世界！"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
}
