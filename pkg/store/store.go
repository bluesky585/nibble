// Package store persists chunk embeddings and searches by cosine similarity.
package store

import (
	"fmt"
	"sort"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/embed"
)

// Record is a chunk plus its embedding.
type Record struct {
	Chunk  chunk.Chunk `json:"chunk"`
	Vector []float64   `json:"vector"`
}

// Hit is a search result.
type Hit struct {
	Record Record  `json:"record"`
	Score  float64 `json:"score"`
}

// Store upserts records and runs brute-force cosine search.
type Store interface {
	Upsert(records []Record) error
	Search(query []float64, k int) ([]Hit, error)
}

// Index embeds chunks in one batch and writes them to st.
// Context is prefixed onto the embedded text when present.
func Index(st Store, emb embed.Embedder, chunks []chunk.Chunk) error {
	if st == nil {
		return fmt.Errorf("store is required")
	}
	if emb == nil {
		return fmt.Errorf("embedder is required")
	}
	if len(chunks) == 0 {
		return nil
	}

	texts := make([]string, len(chunks))
	for i, ch := range chunks {
		if ch.Context != "" {
			texts[i] = ch.Context + ch.Text
		} else {
			texts[i] = ch.Text
		}
	}
	vecs, err := emb.Embed(texts)
	if err != nil {
		return err
	}
	if len(vecs) != len(chunks) {
		return fmt.Errorf("embedder returned %d vectors for %d chunks", len(vecs), len(chunks))
	}

	records := make([]Record, len(chunks))
	for i := range chunks {
		records[i] = Record{Chunk: chunks[i], Vector: vecs[i]}
	}
	return st.Upsert(records)
}

func searchRecords(records []Record, query []float64, k int) ([]Hit, error) {
	if k <= 0 {
		return nil, fmt.Errorf("k must be > 0, got %d", k)
	}
	hits := make([]Hit, 0, len(records))
	for _, rec := range records {
		hits = append(hits, Hit{Record: rec, Score: embed.Cosine(query, rec.Vector)})
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
	return hits[:k], nil
}
