package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/bluesky585/nibble/pkg/embed"
	"github.com/bluesky585/nibble/pkg/store"
)

// runQuery searches the index at path and writes the top k hits as JSON.
//
// The query is embedded as a single text with the same embedder that
// built the index, so a query embedded by a different model simply
// scores badly rather than failing. The index must match -embedder.
// Under bm25 scoring the embedder is not called at all: ranking is term
// overlap over the index texts, which needs no vector model.
// src narrows the search to one source when set: nil searches
// everything, a pointer to "" filters to the records indexed without a
// source.
func runQuery(query, path, scoringName string, hybridWeight float64, k int, src *string, emb embed.Embedder, stdout, stderr io.Writer) int {
	if path == "" {
		fmt.Fprintln(stderr, "-query needs -index")
		return 2
	}
	if k <= 0 {
		fmt.Fprintf(stderr, "-k must be > 0, got %d\n", k)
		return 2
	}

	st, closeIndex, err := openIndex(path)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer closeIndex()

	var hits []store.Hit
	switch scoringName {
	case "dense":
		if src != nil {
			hits, err = denseHitsFiltered(st, query, k, emb, src)
		} else {
			hits, err = denseHits(st, query, k, emb)
		}
	case "bm25":
		hits, err = bm25Hits(st, query, src, k)
	case "hybrid":
		if hybridWeight < 0 || hybridWeight > 1 {
			fmt.Fprintf(stderr, "-hybrid-weight must be in [0, 1], got %v\n", hybridWeight)
			return 2
		}
		hits, err = hybridHits(st, query, k, emb, hybridWeight, src)
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

// filtered returns the corpus narrowed to src when a filter is set
// (src non-nil), or the whole corpus unchanged when it is not. Ranking
// then runs over the survivors, so k counts filtered hits. A pointer
// separates "no filter" from "filter to the empty source", the records
// indexed without a source.
func filtered(st store.Store, src *string) ([]store.Record, error) {
	records, err := store.Records(st)
	if err != nil {
		return nil, err
	}
	if src == nil {
		return records, nil
	}
	return store.FilterSource(records, *src), nil
}

// embedOne runs the embedder over the single query text.
func embedOne(emb embed.Embedder, query string) ([]float64, error) {
	vecs, err := emb.Embed([]string{query})
	if err != nil {
		return nil, err
	}
	if len(vecs) != 1 {
		return nil, fmt.Errorf("embedder returned %d vectors for one query", len(vecs))
	}
	return vecs[0], nil
}

// denseHits is the cosine ranking, as the store has always returned it.
func denseHits(st store.Store, query string, k int, emb embed.Embedder) ([]store.Hit, error) {
	vec, err := embedOne(emb, query)
	if err != nil {
		return nil, err
	}
	return st.Search(vec, k)
}

// denseHitsFiltered is the cosine ranking over one source's records.
// It reads the corpus instead of calling st.Search so the filter lands
// before scoring and k counts filtered hits.
func denseHitsFiltered(st store.Store, query string, k int, emb embed.Embedder, src *string) ([]store.Hit, error) {
	records, err := filtered(st, src)
	if err != nil {
		return nil, err
	}
	vec, err := embedOne(emb, query)
	if err != nil {
		return nil, err
	}
	scores, err := store.RankScores(records, query, vec, store.RankDense, 0)
	if err != nil {
		return nil, err
	}
	return store.Rank(records, scores, k), nil
}

// bm25Hits ranks by term overlap over every record in the index. The
// chunk texts, not the vectors, are the corpus.
func bm25Hits(st store.Store, query string, src *string, k int) ([]store.Hit, error) {
	records, err := filtered(st, src)
	if err != nil {
		return nil, err
	}
	scores, err := store.RankScores(records, query, nil, store.RankBM25, 0)
	if err != nil {
		return nil, err
	}
	return store.Rank(records, scores, k), nil
}

// hybridHits blends the cosine ranking with BM25 through the shared
// ranking layer.
func hybridHits(st store.Store, query string, k int, emb embed.Embedder, weight float64, src *string) ([]store.Hit, error) {
	records, err := filtered(st, src)
	if err != nil {
		return nil, err
	}
	vec, err := embedOne(emb, query)
	if err != nil {
		return nil, err
	}
	scores, err := store.RankScores(records, query, vec, store.RankHybrid, weight)
	if err != nil {
		return nil, err
	}
	return store.Rank(records, scores, k), nil
}
