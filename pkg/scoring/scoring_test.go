package scoring

import (
	"math"
	"strings"
	"testing"

	"github.com/bluesky585/nibble/pkg/chunk"
)

// doc builds a corpus of one-token chunks for scoring tests.
func doc(text string) chunk.Chunk {
	ch, err := chunk.New(text, 0, len([]rune(text)), len([]rune(text)))
	if err != nil {
		panic(err)
	}
	return ch
}

// A document holding the query terms ranks above one that does not,
// and a rarer term lifts a document higher than a common one.
func TestBM25RanksTermsAndRarity(t *testing.T) {
	t.Parallel()

	docs := []chunk.Chunk{
		doc("the cat sat"),
		doc("the dog ran"),
		doc("the fox napped"),
		doc("cat cat cat"),
	}
	bm := NewBM25(docs, WordTerms)

	// Both terms appear once in the first document, so term frequency
	// and document length are equal and only idf differs: "cat" is in
	// two of four documents, "the" in three, so the rarer term must
	// outscore the common one.
	cat := bm.Score(0, "cat")
	the := bm.Score(0, "the")
	if cat <= the {
		t.Fatalf("cat score %v <= the score %v; rarity did not lift the score", cat, the)
	}
	// A term absent from the whole corpus scores 0.
	if got := bm.Score(0, "zebra"); got != 0 {
		t.Fatalf("absent term scored %v, want 0", got)
	}
}

// A term that appears in every document carries the smallest weight the
// formula gives — it cannot vanish entirely, because with a
// one-document index every term is ubiquitous and a hard zero there
// would blank every query — but it must score below any rarer term.
func TestBM25CommonTermIsWeak(t *testing.T) {
	t.Parallel()

	docs := []chunk.Chunk{doc("cat runs"), doc("cat walks"), doc("cat sleeps"), doc("dog naps")}
	bm := NewBM25(docs, WordTerms)
	ubiquitous := bm.Score(0, "cat")
	rare := bm.Score(0, "runs")
	if ubiquitous <= 0 {
		t.Fatalf("ubiquitous term scored %v, want > 0", ubiquitous)
	}
	if ubiquitous >= rare {
		t.Fatalf("ubiquitous %v >= rare %v; idf did not demote it", ubiquitous, rare)
	}
}

// The query is split into terms; each contributes independently, and an
// empty or all-punctuation query contributes nothing.
func TestBM25QuerySplitting(t *testing.T) {
	t.Parallel()

	docs := []chunk.Chunk{doc("the cat sat"), doc("the dog ran")}
	bm := NewBM25(docs, WordTerms)
	one := bm.Score(0, "cat")
	both := bm.Score(0, "cat sat")
	if both <= one {
		t.Fatalf("two terms scored %v, want more than one term's %v", both, one)
	}
	if got := bm.Score(0, ""); got != 0 {
		t.Fatalf("empty query scored %v, want 0", got)
	}
	if got := bm.Score(0, "... !!!"); got != 0 {
		t.Fatalf("punctuation-only query scored %v, want 0", got)
	}
}

// The embed path scores Context too: a CSV row's column names are
// retrieval signal, and the dense embedder sees them through
// EmbedText, so the sparse scorer must match that.
func TestBM25ScoresEmbedText(t *testing.T) {
	t.Parallel()

	ch := doc("apple,red")
	ch.Context = "name,color"
	docs := []chunk.Chunk{ch, doc("banana,yellow")}
	bm := NewBM25(docs, WordTerms)
	// "color" appears only in the Context of the first chunk.
	if got := bm.Score(0, "color"); got <= 0 {
		t.Fatalf("context term scored %v, want > 0", got)
	}
	if got := bm.Score(1, "color"); got != 0 {
		t.Fatalf("scored %v for a chunk without the context term, want 0", got)
	}
}

// Scoring an empty corpus, or a corpus of empty chunks, must not
// divide by zero.
func TestBM25EmptyCorpus(t *testing.T) {
	t.Parallel()

	bm := NewBM25(nil, WordTerms)
	if got := bm.Score(0, "cat"); got != 0 {
		t.Fatalf("empty corpus scored %v, want 0", got)
	}
	bm = NewBM25([]chunk.Chunk{doc("")}, WordTerms)
	if got := bm.Score(0, "cat"); got != 0 {
		t.Fatalf("empty-chunk corpus scored %v, want 0", got)
	}
}

// WordTerms lowercases and strips punctuation, so "Cat" and "cat,"
// match a "cat" term. Terms are never empty.
func TestWordTerms(t *testing.T) {
	t.Parallel()

	got := WordTerms("The cat, sat!")
	want := []string{"the", "cat", "sat"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v want %v", got, want)
	}
}

// Hybrid at weight 1 is exactly the dense ranking, at 0 exactly the
// sparse one, and in between prefers a document that both paths like.
func TestHybridWeights(t *testing.T) {
	t.Parallel()

	dense := []float64{0.9, 0.1}
	docs := []chunk.Chunk{doc("cat cat cat"), doc("dog runs fast")}
	h, err := NewHybrid(dense, NewBM25(docs, WordTerms), 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := h.Score(0, "cat"); math.Abs(got-0.9) > 1e-9 {
		t.Fatalf("w=1 scored %v, want the dense 0.9", got)
	}
	h, err = NewHybrid(dense, NewBM25(docs, WordTerms), 0)
	// At w=0 the score is sparse alone. The first document repeats the
	// query term, so its BM25 weight is positive; the second shares no
	// terms and scores 0.
	if got := h.Score(0, "cat"); got <= 0 {
		t.Fatalf("w=0 scored %v, want the sparse weight of a repeated term", got)
	}
	if got := h.Score(1, "cat"); got != 0 {
		t.Fatalf("w=0 doc 1 scored %v, want 0 (no shared terms)", got)
	}
	// Weights outside [0,1] are rejected.
	if _, err := NewHybrid(dense, NewBM25(docs, WordTerms), 1.5); err == nil {
		t.Fatal("weight 1.5 accepted")
	}
}

// Score must not panic on an empty ranking and out-of-range indexes.
func TestHybridBounds(t *testing.T) {
	t.Parallel()

	h, err := NewHybrid(nil, NewBM25(nil, WordTerms), 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if got := h.Score(0, "cat"); got != 0 {
		t.Fatalf("empty hybrid scored %v, want 0", got)
	}
}
