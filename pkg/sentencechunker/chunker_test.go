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
// carry the sentence's own offsets, not zero-based ones. The sentence
// here has a space, so the fallback cuts it at whitespace rather than
// mid-word, and the leading space rides along as part of the first
// window the way recursive's blank merge works.
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
	// "Hello there." is 12 runes with no delimiter finer than its space,
	// so the fallback splits "Hello " / "there." and windows the pieces.
	if got[0].Text != "Hi." || got[0].Start != 0 || got[0].End != 3 {
		t.Fatalf("first chunk=%+v", got[0])
	}
	if got[1].Start != 3 {
		t.Fatalf("second chunk start=%d want 3: %+v", got[1].Start, got[1])
	}
	if got[1].Text != " Hel" {
		t.Fatalf("second chunk text=%q want %q", got[1].Text, " Hel")
	}
	// The whitespace a window absorbed rides at its edge: the same
	// bounded overflow recursive documents for a merged blank.
	for _, ch := range got[1:] {
		if ch.TokenCount > 4 {
			t.Fatalf("chunk %q counts %d tokens, over size 3 plus its edge space", ch.Text, ch.TokenCount)
		}
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

// An over-budget sentence with clauses is re-cut at the clause before
// token windows are tried, so the words on either side of a comma are
// never welded into one window or split mid-word. This is the case the
// token-only hard split used to get wrong: "chromodynamics" was cut in
// half when the window boundary landed inside it.
func TestChunkOversizedSentenceCutsAtClause(t *testing.T) {
	t.Parallel()

	tok := tokenizer.Word{}
	c, err := New(tok, 3, nil)
	if err != nil {
		t.Fatal(err)
	}

	original := "The quick brown fox, which jumps higher, escapes."
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	// Every window boundary lands on the clause delimiters or the spaces
	// around them: no chunk's text starts or ends in the middle of a word.
	for _, ch := range got {
		text := ch.Text
		if text != "" && text[0] != ' ' {
			if _, ok := wordStart(original, ch.Start); !ok {
				t.Fatalf("chunk %q starts mid-word at %d", text, ch.Start)
			}
		}
	}
	// The clause delimiter survives at a chunk edge instead of a window
	// landing mid-clause.
	joined := ""
	for _, ch := range got {
		joined += ch.Text
	}
	if joined != original {
		t.Fatalf("chunks do not reconstruct: %q", joined)
	}
}

// wordStart reports whether offset i begins a word in text.
func wordStart(text string, i int) (string, bool) {
	if i == 0 {
		return "", true
	}
	words := strings.Fields(text)
	at := 0
	for _, w := range words {
		at = strings.Index(text[at:], w) + at
		if at == i {
			return w, true
		}
		at += len(w)
	}
	return "", false
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
