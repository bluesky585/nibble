package store

import (
	"math"
	"testing"

	"github.com/bluesky585/nibble/pkg/chunk"
)

func sparseRec(text string) Record {
	ch, err := chunk.New(text, 0, len([]rune(text)), len([]rune(text)))
	if err != nil {
		panic(err)
	}
	return Record{Chunk: ch}
}

// RankScoresSparse with a supplied index must land exactly where
// RankScores — which builds its own — lands: same statistics, same
// scores, to float rounding.
func TestRankScoresSparseMatchesRankScores(t *testing.T) {
	t.Parallel()

	records := []Record{
		sparseRec("the cat sat on the mat"),
		sparseRec("dogs bark loudly at the cat"),
		sparseRec("quarks feel the strong force"),
		sparseRec("cat cat cat dog"),
	}
	for _, q := range []string{"cat", "cat quarks", "absent"} {
		direct, err := RankScores(records, q, nil, RankBM25, 0)
		if err != nil {
			t.Fatal(err)
		}
		cached, err := RankScoresSparse(records, SparseIndex(records), q, nil, RankBM25, 0)
		if err != nil {
			t.Fatal(err)
		}
		for i := range direct {
			if math.Abs(direct[i]-cached[i]) > 1e-9 {
				t.Fatalf("query %q record %d: direct %v cached %v", q, i, direct[i], cached[i])
			}
		}
	}
}

// Hybrid with a supplied index blends the same two sides RankScores
// blends.
func TestRankScoresSparseHybrid(t *testing.T) {
	t.Parallel()

	records := []Record{
		{Chunk: mustTextChunk("cat purrs softly"), Vector: []float32{1, 0, 0}},
		{Chunk: mustTextChunk("quarks feel force"), Vector: []float32{0, 1, 0}},
	}
	qvec := []float32{1, 0, 0}
	direct, err := RankScores(records, "cat", qvec, RankHybrid, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	cached, err := RankScoresSparse(records, SparseIndex(records), "cat", qvec, RankHybrid, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	for i := range direct {
		if math.Abs(direct[i]-cached[i]) > 1e-9 {
			t.Fatalf("record %d: direct %v cached %v", i, direct[i], cached[i])
		}
	}
}

func mustTextChunk(text string) chunk.Chunk {
	ch, err := chunk.New(text, 0, len([]rune(text)), len([]rune(text)))
	if err != nil {
		panic(err)
	}
	return ch
}
