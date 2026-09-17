package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/embed"
	"github.com/bluesky585/nibble/pkg/scoring"
	"github.com/bluesky585/nibble/pkg/store"
)

// runQuery searches the index at path and writes the top k hits as JSON.
//
// The query is embedded as a single text with the same embedder that
// built the index, so a query embedded by a different model simply
// scores badly rather than failing. The index must match -embedder.
// Under bm25 scoring the embedder is not called at all: ranking is term
// overlap over the index texts, which needs no vector model.
func runQuery(query, path, scoringName string, hybridWeight float64, k int, emb embed.Embedder, stdout, stderr io.Writer) int {
	if path == "" {
		fmt.Fprintln(stderr, "-query needs -index")
		return 2
	}
	if k <= 0 {
		fmt.Fprintf(stderr, "-k must be > 0, got %d\n", k)
		return 2
	}

	st, err := store.OpenJSONL(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	var hits []store.Hit
	switch scoringName {
	case "dense":
		hits, err = denseHits(st, query, k, emb, stderr)
	case "bm25":
		hits, err = bm25Hits(st, query, k)
	case "hybrid":
		if hybridWeight < 0 || hybridWeight > 1 {
			fmt.Fprintf(stderr, "-hybrid-weight must be in [0, 1], got %v\n", hybridWeight)
			return 2
		}
		hits, err = hybridHits(st, query, k, emb, hybridWeight, stderr)
	default:
		fmt.Fprintf(stderr, "unknown -scoring %q: use dense, bm25, or hybrid\n", scoringName)
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if hits == nil {
		hits = []store.Hit{}
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(hits); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// denseHits is the cosine ranking, as the store has always returned it.
func denseHits(st *store.JSONL, query string, k int, emb embed.Embedder, stderr io.Writer) ([]store.Hit, error) {
	vecs, err := emb.Embed([]string{query})
	if err != nil {
		return nil, err
	}
	if len(vecs) != 1 {
		return nil, fmt.Errorf("embedder returned %d vectors for one query", len(vecs))
	}
	return st.Search(vecs[0], k)
}

// bm25Hits ranks by term overlap over every record in the index. The
// chunk texts, not the vectors, are the corpus.
func bm25Hits(st *store.JSONL, query string, k int) ([]store.Hit, error) {
	records := st.Records()
	if len(records) == 0 {
		return nil, nil
	}
	docs := make([]chunk.Chunk, len(records))
	for i, rec := range records {
		docs[i] = rec.Chunk
	}
	ranker := scoring.NewBM25(docs, scoring.WordTerms)
	return rankRecords(records, func(i int) float64 { return ranker.Score(i, query) }, k), nil
}

// hybridHits blends the cosine ranking with BM25. The dense side keeps
// its native score; the sparse side is normalized to [0, 1] over the
// index before blending, because BM25 scores are unbounded while cosine
// is not, and an unbounded side would dominate the blend at any weight
// in between.
func hybridHits(st *store.JSONL, query string, k int, emb embed.Embedder, weight float64, stderr io.Writer) ([]store.Hit, error) {
	records := st.Records()
	if len(records) == 0 {
		return nil, nil
	}

	vecs, err := emb.Embed([]string{query})
	if err != nil {
		return nil, err
	}
	if len(vecs) != 1 {
		return nil, fmt.Errorf("embedder returned %d vectors for one query", len(vecs))
	}
	denseScores, err := denseScoresFor(records, vecs[0])
	if err != nil {
		return nil, err
	}

	docs := make([]chunk.Chunk, len(records))
	for i, rec := range records {
		docs[i] = rec.Chunk
	}
	ranker := scoring.NewBM25(docs, scoring.WordTerms)
	sparseScores := make([]float64, len(records))
	maxSparse := 0.0
	for i := range records {
		sparseScores[i] = ranker.Score(i, query)
		if sparseScores[i] > maxSparse {
			maxSparse = sparseScores[i]
		}
	}
	if maxSparse > 0 {
		for i := range sparseScores {
			sparseScores[i] /= maxSparse
		}
	}

	merged := make([]float64, len(records))
	for i := range merged {
		merged[i] = weight*denseScores[i] + (1-weight)*sparseScores[i]
	}
	return rankRecords(records, func(i int) float64 { return merged[i] }, k), nil
}

// denseScoresFor scores every record against the query vector. A
// dimension mismatch is an error here rather than a silent zero, since
// the caller has already decided the embedder applies.
func denseScoresFor(records []store.Record, query []float64) ([]float64, error) {
	if len(records) > 0 && len(query) != len(records[0].Vector) {
		return nil, fmt.Errorf(
			"query has %d dimensions but the index has %d; the query must use the same embedder as the index",
			len(query), len(records[0].Vector),
		)
	}
	out := make([]float64, len(records))
	for i, rec := range records {
		out[i] = embed.Cosine(query, rec.Vector)
	}
	return out, nil
}

// rankRecords orders records by score descending and takes k. Ties
// break on chunk start, matching the store's cosine ranking.
func rankRecords(records []store.Record, score func(int) float64, k int) []store.Hit {
	hits := make([]store.Hit, len(records))
	for i, rec := range records {
		hits[i] = store.Hit{Record: rec, Score: score(i)}
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
