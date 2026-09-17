package sentencechunker

import (
	"strings"
	"testing"

	"github.com/bluesky585/nibble/internal/assertchunk"
	"github.com/bluesky585/nibble/pkg/chunk"
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

// A sentence over budget on its own is cut into token windows instead of
// being emitted whole, so no chunk exceeds size.
func TestChunkOversizedSentenceIsHardSplit(t *testing.T) {
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
	assertchunk.Split(t, original, got)
	// "Hello." is 6 runes > 4, so it becomes "Hell" + "o.".
	if len(got) != 3 {
		t.Fatalf("len=%d want 3, got %+v", len(got), got)
	}
	if got[0].Text != "Hell" || got[1].Text != "o." {
		t.Fatalf("hard split wrong: %+v", got[:2])
	}
	assertNoChunkOver(t, got, 4)
}

// The oversized sentence keeps its place in the document: its windows
// carry the sentence's own offsets, not zero-based ones.
func TestChunkOversizedSentenceOffsets(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 3, nil)
	if err != nil {
		t.Fatal(err)
	}

	original := "Hi. Hello there."
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	assertNoChunkOver(t, got, 3)
	// "Hi." is 3 runes, so the next sentence starts at rune 3. Its
	// windows must carry that offset, not zero.
	if got[0].Text != "Hi." || got[0].Start != 0 || got[0].End != 3 {
		t.Fatalf("first chunk=%+v", got[0])
	}
	if got[1].Start != 3 {
		t.Fatalf("second chunk start=%d want 3: %+v", got[1].Start, got[1])
	}
	if got[1].Text != " He" {
		t.Fatalf("second chunk text=%q want %q", got[1].Text, " He")
	}
}

// A run of text with no sentence delimiter is still bounded by size.
func TestChunkNoDelimiterOverBudget(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 4, nil)
	if err != nil {
		t.Fatal(err)
	}

	original := "abcdefghij"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	assertNoChunkOver(t, got, 4)
	if len(got) != 3 {
		t.Fatalf("len=%d want 3, got %+v", len(got), got)
	}
}

func assertNoChunkOver(t *testing.T, chunks []chunk.Chunk, size int) {
	t.Helper()
	for _, ch := range chunks {
		if ch.TokenCount > size {
			t.Fatalf("chunk %q has %d tokens, over size %d", ch.Text, ch.TokenCount, size)
		}
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

// Text with no delimiter under the budget stays a single chunk.
func TestChunkNoSentenceDelim(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 64, nil)
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

func TestNewMinRunesMergesFragments(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 512, nil, MinRunes(4))
	if err != nil {
		t.Fatal(err)
	}

	// Under "." each abbreviation yields a piece of its own: "e." and
	// "g." are 2 runes each. MinRunes 4 merges them forward, so no
	// fragment becomes a chunk of its own.
	original := "Test this, e.g. the first case. And one more."
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	for _, ch := range got {
		if n := utf8Count(ch.Text); n < 4 {
			t.Fatalf("chunk %q has %d runes, want >= 4", ch.Text, n)
		}
	}
}

func TestNewMinRunesZeroKeepsShortSentences(t *testing.T) {
	t.Parallel()

	// CJK sentences are short and complete; merging them would destroy
	// real boundaries. This is why MinRunes is opt-in, not a default.
	//
	// Asserted at the split layer, not through Chunk: packing merges
	// pieces that fit the budget, so chunk count says nothing about where
	// the scanner cut. A MinRunes of 4 here would fuse 你好。 and 世界！
	// into one piece before packing ever saw them.
	c, err := New(tokenizer.Character{}, 512, nil)
	if err != nil {
		t.Fatal(err)
	}
	original := "你好。世界！"
	pieces, err := split.Text(original, split.Options{
		Delimiters: DefaultDelimiters,
		Attach:     split.AttachPrev,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pieces) != 2 {
		t.Fatalf("pieces=%d want 2, one per sentence: %+v", len(pieces), pieces)
	}

	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
}

func TestNewMinRunesValidation(t *testing.T) {
	t.Parallel()

	_, err := New(tokenizer.Character{}, 8, nil, MinRunes(-1))
	if err == nil || !strings.Contains(err.Error(), "min runes must be >= 0") {
		t.Fatalf("err=%v", err)
	}
}
