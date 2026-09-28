package scoring

import (
	"math"
	"testing"

	"github.com/bluesky585/nibble/pkg/chunk"
)

// Scores must agree with the per-document Score to float rounding: the
// two walk the same postings, Scores just terms the query once and
// skips documents that hold none of its terms.
func TestBM25ScoresMatchesScore(t *testing.T) {
	t.Parallel()

	docs := []chunk.Chunk{
		doc("the cat sat on the mat"),
		doc("dogs bark loudly at the cat next door"),
		doc("quantum chromodynamics is not about cats"),
		doc("quarks feel the strong force"),
		doc("cat cat cat dog"),
	}
	bm := NewBM25(docs, WordTerms)

	for _, q := range []string{"cat", "cat dog quarks", "the", "absentword", "the cat sat"} {
		want := make([]float64, len(docs))
		for i := range docs {
			want[i] = bm.Score(i, q)
		}
		got := bm.Scores(q)
		for i := range want {
			if math.Abs(want[i]-got[i]) > 1e-9 {
				t.Fatalf("Scores(%q)[%d] = %v, Score = %v", q, i, got[i], want[i])
			}
		}
	}
}

// A posting list stays ascending by document, which termFreq's binary
// search relies on. Construction appends in document order, so the
// property is checked rather than enforced with a sort.
func TestBM25PostingsAscending(t *testing.T) {
	t.Parallel()

	docs := []chunk.Chunk{doc("b a b"), doc("a c"), doc("b b a c c")}
	bm := NewBM25(docs, WordTerms)
	for term, list := range bm.postings {
		for i := 1; i < len(list); i++ {
			if list[i].doc <= list[i-1].doc {
				t.Fatalf("term %q postings not ascending: %v", term, list)
			}
		}
	}
	// Spot-check tf against the texts.
	if got := bm.termFreq("b", 0); got != 2 {
		t.Fatalf("tf(b, doc 0) = %d, want 2", got)
	}
	if got := bm.termFreq("c", 2); got != 2 {
		t.Fatalf("tf(c, doc 2) = %d, want 2", got)
	}
	if got := bm.termFreq("a", 1); got != 1 {
		t.Fatalf("tf(a, doc 1) = %d, want 1", got)
	}
	if got := bm.termFreq("b", 1); got != 0 {
		t.Fatalf("tf(b, doc 1) = %d, want 0 (absent)", got)
	}
}

// Context joins the scored text, as it always has: a table chunk's
// header must be findable by the terms it carries.
func TestBM25ContextScored(t *testing.T) {
	t.Parallel()

	ch, err := chunk.New("row body text", 0, 13, 3)
	if err != nil {
		t.Fatal(err)
	}
	ch.Context = "name,color"
	bm := NewBM25([]chunk.Chunk{ch}, WordTerms)
	if bm.Scores("color")[0] == 0 {
		t.Fatal("context terms must be scored")
	}
}
