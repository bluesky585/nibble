package semantic

import (
	"strings"
	"testing"
	"unicode/utf8"

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

// A sentence over budget on its own is cut into token windows, so no
// chunk exceeds size even when similarity would otherwise keep it whole.
func TestChunkOversizedSentenceIsHardSplit(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, embed.Hashing{}, 4, 0)
	if err != nil {
		t.Fatal(err)
	}

	original := "abcdefghij"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) != 3 {
		t.Fatalf("len=%d want 3, got %+v", len(got), got)
	}
	for _, ch := range got {
		if ch.TokenCount > 4 {
			t.Fatalf("chunk %q has %d tokens, over size 4", ch.Text, ch.TokenCount)
		}
	}
	if got[1].Start != 4 {
		t.Fatalf("window offsets must follow the document: %+v", got[1])
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

func TestNewMinRunes(t *testing.T) {
	t.Parallel()

	if _, err := New(tokenizer.Character{}, embed.Hashing{}, 8, 0, MinRunes(-1)); err == nil || !strings.Contains(err.Error(), "min runes must be >= 0") {
		t.Fatalf("err=%v", err)
	}

	// The option reaches the sentence split: with MinRunes 4 the "e." and
	// "g." fragments merge forward, so no emitted chunk is a fragment.
	c, err := New(tokenizer.Character{}, embed.Hashing{}, 512, 0, MinRunes(4))
	if err != nil {
		t.Fatal(err)
	}
	original := "Test this, e.g. the first case. And one more."
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	for _, ch := range got {
		if n := utf8.RuneCountInString(ch.Text); n < 4 {
			t.Fatalf("chunk %q has %d runes, want >= 4", ch.Text, n)
		}
	}
}

// windowTexts is a two-topic document for testing the similarity window.
// Every sentence in a topic shares that topic's anchor word, so adjacent
// sentences inside a topic stay above the threshold. There is exactly one
// topic boundary, and it drops the similarity to zero.
func windowTexts() string {
	a := []string{
		"cats sleep on soft beds.",
		"cats sleep in warm sun.",
		"cats sleep through rainy nights.",
		"cats dream.",
		"cats sleep all day long.",
		"cats sleep near warm fires.",
		"cats sleep after every meal.",
		"cats sleep until noon arrives.",
	}
	b := []string{
		"Quantum entanglement links distant particles.",
		"Quantum superposition defies classical physics.",
		"Quantum computing uses qubits and gates.",
		"Quantum states collapse when measured.",
	}
	return strings.Join(a, "") + strings.Join(b, "")
}

// Adjacent-pair similarity inside a topic jitters: one pair of sentences
// can land below the threshold even though no topic changed there. The
// window averages over several sentences, which smooths that jitter while
// the real boundary still cuts.
//
// Asserted on the boundary counts, not on the split layer: packing merges
// pieces that fit the budget, so chunk count says nothing about where the
// similarity test cut.
func TestSimilarityWindowSmoothsJitter(t *testing.T) {
	t.Parallel()

	original := windowTexts()

	// With a window of 1 the in-topic jitter cuts the topic into pieces;
	// with a window of 3 it does not. minSim 0.35 sits between the
	// smoothed in-topic values and the smoothed boundary value.
	w1, err := New(tokenizer.Character{}, embed.Hashing{}, 512, 0.35)
	if err != nil {
		t.Fatal(err)
	}
	w3, err := New(tokenizer.Character{}, embed.Hashing{}, 512, 0.35, SimilarityWindow(3))
	if err != nil {
		t.Fatal(err)
	}

	splits := func(c Chunker) int {
		t.Helper()
		chunks, err := c.Chunk(original)
		if err != nil {
			t.Fatal(err)
		}
		assertchunk.Split(t, original, chunks)
		return len(chunks)
	}

	got1, got3 := splits(w1), splits(w3)
	if got1 <= got3 {
		t.Fatalf("window 1 should cut more than window 3: w1=%d w3=%d", got1, got3)
	}
	if got3 != 2 {
		t.Fatalf("window 3 should keep each topic whole: got %d chunks, want 2", got3)
	}
}

func TestSimilarityWindowValidation(t *testing.T) {
	t.Parallel()

	_, err := New(tokenizer.Character{}, embed.Hashing{}, 8, 0, SimilarityWindow(0))
	if err == nil || !strings.Contains(err.Error(), "similarity window must be >= 1") {
		t.Fatalf("err=%v", err)
	}
}
