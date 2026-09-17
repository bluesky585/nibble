package store

import (
	"fmt"
	"sort"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/embed"
	"github.com/bluesky585/nibble/pkg/scoring"
)

// Ranking modes shared by the CLI's -scoring and the API's "scoring"
// field. RankDense is cosine over stored vectors, the default; RankBM25
// is term overlap over the indexed texts and needs no embedder at all;
// RankHybrid blends both.
const (
	RankDense  = "dense"
	RankBM25   = "bm25"
	RankHybrid = "hybrid"
)

// ValidScoring reports whether name is a scoring mode Rank accepts.
func ValidScoring(name string) bool {
	switch name {
	case RankDense, RankBM25, RankHybrid:
		return true
	}
	return false
}

// Records returns the whole corpus of st, for the ranking modes that
// read texts rather than search vectors. The Store interface cannot
// expose it (the JSONL shape predates the error return), so the concrete
// stores are told apart here; a store that grows later joins this
// switch.
func Records(st Store) ([]Record, error) {
	switch src := st.(type) {
	case *JSONL:
		return src.Records(), nil
	case *SQLite:
		return src.Records()
	default:
		return nil, nil
	}
}

// RankScores scores every record in records against the query, by the
// named mode. dense needs a query vector (the embedder's job); bm25 and
// hybrid need the query text. The sparse side of a hybrid is normalized
// to the corpus maximum first, since BM25 scores are unbounded while
// cosine is not, and an unbounded side would dominate the blend at any
// weight in between. weight is the dense share of a hybrid; it is
// ignored by the other modes.
//
// A dimension mismatch between the query vector and the corpus is an
// error rather than a silent zero, since the caller has already decided
// the embedder applies. Scores across modes are not comparable — each
// mode is its own measure.
func RankScores(records []Record, queryText string, queryVec []float64, mode string, weight float64) ([]float64, error) {
	if !ValidScoring(mode) {
		return nil, fmt.Errorf("unknown scoring %q: use dense, bm25, or hybrid", mode)
	}
	if mode == RankHybrid && (weight < 0 || weight > 1) {
		return nil, fmt.Errorf("hybrid weight must be in [0, 1], got %v", weight)
	}
	if len(records) == 0 {
		return nil, nil
	}

	switch mode {
	case RankDense:
		out := make([]float64, len(records))
		for i, rec := range records {
			out[i] = embed.Cosine(queryVec, rec.Vector)
		}
		return out, nil
	case RankBM25:
		return sparseScores(records, queryText), nil
	default: // RankHybrid
		if len(queryVec) != len(records[0].Vector) {
			return nil, fmt.Errorf(
				"query has %d dimensions but the index has %d; the query must use the same embedder as the index",
				len(queryVec), len(records[0].Vector),
			)
		}
		dense := make([]float64, len(records))
		for i, rec := range records {
			dense[i] = embed.Cosine(queryVec, rec.Vector)
		}
		sparse := sparseScores(records, queryText)
		merged := make([]float64, len(records))
		for i := range merged {
			merged[i] = weight*dense[i] + (1-weight)*sparse[i]
		}
		return merged, nil
	}
}

// sparseScores scores every record with BM25 over the record texts,
// normalized to the corpus maximum so the numbers land in [0, 1].
func sparseScores(records []Record, query string) []float64 {
	docs := make([]chunk.Chunk, len(records))
	for i, rec := range records {
		docs[i] = rec.Chunk
	}
	ranker := scoring.NewBM25(docs, scoring.WordTerms)
	out := make([]float64, len(records))
	maxSparse := 0.0
	for i := range records {
		out[i] = ranker.Score(i, query)
		if out[i] > maxSparse {
			maxSparse = out[i]
		}
	}
	if maxSparse > 0 {
		for i := range out {
			out[i] /= maxSparse
		}
	}
	return out
}

// Rank orders records by score descending and takes k, wrapping the
// scores with their records. Ties break on chunk start, matching the
// store's cosine ranking. It is the shared tail of every ranking mode.
func Rank(records []Record, scores []float64, k int) []Hit {
	hits := make([]Hit, len(records))
	for i, rec := range records {
		hits[i] = Hit{Record: rec, Score: scores[i]}
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].Record.Chunk.Start < hits[j].Record.Chunk.Start
		}
		return hits[i].Score > hits[j].Score
	})
	if k > len(hits) {
		k = len(hits)
	}
	return hits[:k]
}
