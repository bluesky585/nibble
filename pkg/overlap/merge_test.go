package overlap

import (
	"strings"
	"testing"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

func mustChunk2(t *testing.T, text string, start int) chunk.Chunk {
	t.Helper()
	n := len([]rune(text))
	ch, err := chunk.New(text, start, start+n, n)
	if err != nil {
		t.Fatal(err)
	}
	return ch
}

func TestMergePrefix(t *testing.T) {
	t.Parallel()

	tok := tokenizer.Character{}
	in := []chunk.Chunk{
		mustChunk2(t, "Cats sleep.", 0),
		mustChunk2(t, "Dogs bark.", 11),
	}
	got, err := Merge(in, tok, 3, PrefixMode)
	if err != nil {
		t.Fatal(err)
	}

	// The second chunk's text now starts with the tail of the first:
	// the last 3 characters of "Cats sleep." are "ep.".
	want := "ep.Dogs bark."
	if got[1].Text != want {
		t.Fatalf("text=%q want %q", got[1].Text, want)
	}
	if got[1].Context != "ep." {
		t.Fatalf("context=%q want %q", got[1].Context, "ep.")
	}
	// TokenCount grows by the context, since Text now holds it:
	// "Dogs bark." is 10 tokens, "ep." adds 3.
	if got[1].TokenCount != 13 {
		t.Fatalf("token_count=%d want 13", got[1].TokenCount)
	}
	// Offsets still name the slice of the original document the text
	// came from, not the merged string.
	if got[1].Start != 11 || got[1].End != 21 {
		t.Fatalf("offsets=%d..%d want 11..21", got[1].Start, got[1].End)
	}
	// The first chunk never gains context.
	if got[0].Text != "Cats sleep." || got[0].Context != "" {
		t.Fatalf("first chunk changed: %+v", got[0])
	}
	// The input is not modified in place.
	if in[1].Text != "Dogs bark." {
		t.Fatalf("input mutated: %+v", in[1])
	}
}

func TestMergeSuffix(t *testing.T) {
	t.Parallel()

	tok := tokenizer.Character{}
	in := []chunk.Chunk{
		mustChunk2(t, "Cats sleep.", 0),
		mustChunk2(t, "Dogs bark.", 11),
	}
	got, err := Merge(in, tok, 3, SuffixMode)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Text != "Cats sleep.Dog" {
		t.Fatalf("text=%q want %q", got[0].Text, "Cats sleep.Dog")
	}
	if got[0].Context != "Dog" {
		t.Fatalf("context=%q want %q", got[0].Context, "Dog")
	}
	if got[0].TokenCount != 14 {
		t.Fatalf("token_count=%d want 14", got[0].TokenCount)
	}
	// The last chunk never gains context.
	if got[1].Text != "Dogs bark." || got[1].Context != "" {
		t.Fatalf("last chunk changed: %+v", got[1])
	}
}

func TestMergeJustified(t *testing.T) {
	t.Parallel()

	tok := tokenizer.Character{}
	in := []chunk.Chunk{
		mustChunk2(t, "Cats sleep.", 0),
		mustChunk2(t, "Dogs bark.", 11),
		mustChunk2(t, "Birds sing.", 21),
	}
	got, err := Merge(in, tok, 3, JustifiedMode)
	if err != nil {
		t.Fatal(err)
	}

	// First chunk: suffix only. Middle: both sides. Last: prefix only.
	// The middle chunk's prefix is the tail of chunk 0 ("ep.") and its
	// suffix is the head of chunk 2 ("Bir").
	if got[0].Text != "Cats sleep.Dog" {
		t.Fatalf("first=%q", got[0].Text)
	}
	if got[1].Text != "ep.Dogs bark.Bir" {
		t.Fatalf("middle=%q", got[1].Text)
	}
	if got[2].Text != "rk.Birds sing." {
		t.Fatalf("last=%q", got[2].Text)
	}
	if got[1].Context != "ep.Bir" {
		t.Fatalf("middle context=%q want %q", got[1].Context, "ep.Bir")
	}
}

// Merge breaks the reconstruct guarantee on purpose: merged text repeats
// text that other chunks own. The test says so, so nobody "fixes" it.
func TestMergeDoesNotReconstruct(t *testing.T) {
	t.Parallel()

	tok := tokenizer.Character{}
	original := "Cats sleep. Dogs bark. Birds sing."
	in := []chunk.Chunk{
		mustChunk2(t, "Cats sleep.", 0),
		mustChunk2(t, " Dogs bark.", 11),
		mustChunk2(t, " Birds sing.", 21),
	}
	got, err := Merge(in, tok, 4, JustifiedMode)
	if err != nil {
		t.Fatal(err)
	}
	var joined string
	for _, ch := range got {
		joined += ch.Text
	}
	if joined == original {
		t.Fatal("merged text should not reconstruct the original; that is the documented trade")
	}
}

// Context set before the merge is kept in front, like Prefix and Suffix do.
func TestMergeKeepsExistingContext(t *testing.T) {
	t.Parallel()

	tok := tokenizer.Character{}
	in := []chunk.Chunk{mustChunk2(t, "Cats sleep.", 0), mustChunk2(t, "Dogs bark.", 11)}
	in[1].Context = "HDR: "
	got, err := Merge(in, tok, 2, PrefixMode)
	if err != nil {
		t.Fatal(err)
	}
	// Context gains the overlap tail; Text is context + original text.
	// n=2, so the overlap is the last 2 characters of chunk 0: "p.".
	if !strings.HasPrefix(got[1].Text, "p.") {
		t.Fatalf("text should start with the overlap, got %q", got[1].Text)
	}
}

func TestMergeValidation(t *testing.T) {
	t.Parallel()

	tok := tokenizer.Character{}
	if _, err := Merge(nil, nil, 2, PrefixMode); err == nil || !strings.Contains(err.Error(), "tokenizer is required") {
		t.Fatalf("err=%v", err)
	}
	if _, err := Merge(nil, tok, -1, PrefixMode); err == nil || !strings.Contains(err.Error(), "n must be >= 0") {
		t.Fatalf("err=%v", err)
	}
	if _, err := Merge(nil, tok, 2, "bogus"); err == nil || !strings.Contains(err.Error(), "unknown mode") {
		t.Fatalf("err=%v", err)
	}
	if _, err := Merge(nil, tok, 0, PrefixMode); err != nil {
		t.Fatalf("empty input should be nil, err=%v", err)
	}
}

func TestMergeZero(t *testing.T) {
	t.Parallel()

	tok := tokenizer.Character{}
	in := []chunk.Chunk{mustChunk2(t, "Cats sleep.", 0), mustChunk2(t, "Dogs bark.", 11)}
	got, err := Merge(in, tok, 0, JustifiedMode)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Text != "Cats sleep." || got[1].Text != "Dogs bark." {
		t.Fatalf("n=0 should change nothing: %+v", got)
	}
}
