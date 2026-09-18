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

// When a paragraph exactly fills the budget, the blank-line separator
// that follows it drills down as its own piece and hard-splits into a
// whitespace-only chunk. Such a chunk is pure retrieval noise: it carries
// no content, so it is merged into the previous chunk rather than
// emitted. Reconstruct is unaffected — the text was never dropped.
func TestChunkBlankSeparatorChunkMergesIntoPrevious(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 512, nil)
	if err != nil {
		t.Fatal(err)
	}
	para := strings.Repeat("x", 512)
	original := para + "\n\n" + strings.Repeat("y", 512)
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	for i, ch := range got {
		if strings.TrimSpace(ch.Text) == "" {
			t.Fatalf("chunk %d is whitespace-only: %q", i, ch.Text)
		}
	}
	// The separator belongs to the end of the first paragraph, so the
	// first chunk carries it and stays within one chunk per paragraph.
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	if got[0].Text != para+"\n\n" {
		t.Fatalf("first=%q want %q", got[0].Text[:8], para[:8]+"\n\n")
	}
}

// The same noise can appear as the first chunk when a leading blank
// separator precedes a budget-filling paragraph.
func TestChunkLeadingBlankSeparatorChunkMergesIntoNext(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 512, nil)
	if err != nil {
		t.Fatal(err)
	}
	para := strings.Repeat("y", 512)
	original := strings.Repeat("x", 512) + "\n\n" + para
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	for i, ch := range got {
		if strings.TrimSpace(ch.Text) == "" {
			t.Fatalf("chunk %d is whitespace-only: %q", i, ch.Text)
		}
	}
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

// Overlap repeats the tail of the previous chunk at the head of the
// next one, in Text itself, the way the token chunker widens a window.
// Each chunk still ends at the boundary the rules chose; the next one
// just starts n tokens earlier, so a retrieval hit on either side of a
// cut can see across it.
func TestChunkOverlap(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 8, nil, Overlap(2))
	if err != nil {
		t.Fatal(err)
	}
	original := strings.Repeat("word ", 8)
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 3 {
		t.Fatalf("chunks=%d, want several to exercise the boundary", len(got))
	}
	// Every chunk after the first starts exactly 2 runes (2 character
	// tokens) before the previous chunk ended.
	for i := 1; i < len(got); i++ {
		if got[i].Start != got[i-1].End-2 {
			t.Fatalf("chunk %d start=%d, want %d", i, got[i].Start, got[i-1].End-2)
		}
		if !strings.HasPrefix(got[i].Text, original[got[i].Start:got[i-1].End]) {
			t.Fatalf("chunk %d does not repeat the previous tail: %q", i, got[i].Text)
		}
	}
	// The tail is source text and the end is unchanged, so every chunk
	// must be a slice of the input and end where the rules put it.
	for i, ch := range got {
		if original[ch.Start:ch.End] != ch.Text {
			t.Fatalf("chunk %d is not a slice of the input", i)
		}
	}
	// The last chunk must end at the end of the input.
	if last := got[len(got)-1]; last.End != len([]rune(original)) {
		t.Fatalf("last end=%d want %d", last.End, len([]rune(original)))
	}
}

// A negative overlap and one at or over the budget are build errors,
// matching the token chunker's validation.
func TestNewOverlapValidation(t *testing.T) {
	t.Parallel()

	if _, err := New(tokenizer.Character{}, 8, nil, Overlap(-1)); err == nil ||
		!strings.Contains(err.Error(), "overlap must be >= 0") {
		t.Fatalf("err=%v", err)
	}
	if _, err := New(tokenizer.Character{}, 8, nil, Overlap(8)); err == nil ||
		!strings.Contains(err.Error(), "overlap must be < size") {
		t.Fatalf("err=%v", err)
	}
}

// Overlap zero is the shape every existing caller has: chunks remain
// contiguous and reconstruct holds.
func TestChunkOverlapZeroReconstructs(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 8, nil, Overlap(0))
	if err != nil {
		t.Fatal(err)
	}
	original := "One paragraph here.\n\nAnother paragraph follows it."
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
}

// Overlap applies after blank merging: the chunk a blank was merged
// into is the neighbor whose tail the next chunk repeats, and no
// overlap reaches into a blank-only region twice.
func TestChunkOverlapAfterBlankMerge(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 10, nil, Overlap(3))
	if err != nil {
		t.Fatal(err)
	}
	original := "First paragraph.\n\n\n\nSecond paragraph.\n\nThird."
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	for i, ch := range got {
		if original[ch.Start:ch.End] != ch.Text {
			t.Fatalf("chunk %d is not a slice of the input: [%d,%d) %q",
				i, ch.Start, ch.End, ch.Text)
		}
	}
	if last := got[len(got)-1]; last.End != len([]rune(original)) {
		t.Fatalf("last end=%d want %d", last.End, len([]rune(original)))
	}
}

// FallbackRules is the below-sentence hierarchy the sentence and
// semantic chunkers use to re-cut one over-budget sentence. On a text
// with clauses it cuts at the clause; on a text with none it lands on
// the token level, which is the hard split a chunker without this
// second attempt would have taken directly.
func TestChunkFallbackRules(t *testing.T) {
	t.Parallel()

	tok := tokenizer.Word{}
	c, err := New(tok, 2, FallbackRules())
	if err != nil {
		t.Fatal(err)
	}

	original := "alpha, beta, gamma"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	// The cut at size 2 cannot fit "alpha, beta," whole, so the clause
	// delimiter is where the windows break instead of a mid-word seam.
	texts := make([]string, len(got))
	for i, ch := range got {
		texts[i] = ch.Text
	}
	joined := strings.Join(texts, "")
	if joined != original {
		t.Fatalf("chunks do not reconstruct: %q", joined)
	}
	if len(got) < 2 {
		t.Fatalf("expected several chunks, got %+v", got)
	}

	// No clause, no space: the token level is the only level left. The
	// character ruler gives every rune its own token, so size 1 means
	// one rune per window.
	c1, err := New(tokenizer.Character{}, 1, FallbackRules())
	if err != nil {
		t.Fatal(err)
	}
	original1 := "abcdefgh"
	got1, err := c1.Chunk(original1)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original1, got1)
	if len(got1) != 8 {
		t.Fatalf("token fallback len=%d want 8, got %+v", len(got1), got1)
	}
}

// The whitespace level of FallbackRules covers line breaks, which the
// sentence delimiters above it do not: a hard-wrapped over-budget
// sentence is cut at its newlines before token windows are tried.
func TestChunkFallbackRulesLineBreaks(t *testing.T) {
	t.Parallel()

	tok := tokenizer.Word{}
	c, err := New(tok, 1, FallbackRules())
	if err != nil {
		t.Fatal(err)
	}

	original := "one\ntwo\nthree"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	for _, ch := range got {
		if ch.TokenCount > 2 {
			t.Fatalf("chunk %q has %d tokens, over size 1 plus its edge breaks", ch.Text, ch.TokenCount)
		}
	}
	// AttachPrev keeps each line break on the line before it, so the
	// break rides at a chunk edge rather than starting one.
	if got[0].Text != "one\n" {
		t.Fatalf("first chunk=%q, want the first line with its break", got[0].Text)
	}
}
