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

// embedBatchSize caps how many texts go into one Embedder.Embed call. The
// OpenAI embeddings API rejects a request that carries too many inputs, so a
// large run is split into bounded batches rather than one unbounded call or
// one call per chunk.
const embedBatchSize = 256

// Embed fills in each chunk's Embedding and returns the chunks. Unlike Index
// it persists nothing, so the vectors can be inspected without a store.
// Context is prefixed onto the embedded text when present.
func Embed(emb embed.Embedder, chunks []chunk.Chunk) ([]chunk.Chunk, error) {
	if emb == nil {
		return nil, fmt.Errorf("embedder is required")
	}
	if len(chunks) == 0 {
		return chunks, nil
	}

	out := make([]chunk.Chunk, len(chunks))
	copy(out, chunks)
	for start := 0; start < len(out); start += embedBatchSize {
		end := start + embedBatchSize
		if end > len(out) {
			end = len(out)
		}
		texts := make([]string, end-start)
		for i := start; i < end; i++ {
			texts[i-start] = out[i].EmbedText()
		}
		vecs, err := emb.Embed(texts)
		if err != nil {
			return nil, err
		}
		if len(vecs) != len(texts) {
			return nil, fmt.Errorf("embedder returned %d vectors for %d chunks", len(vecs), len(texts))
		}
		for i := start; i < end; i++ {
			out[i].Embedding = vecs[i-start]
		}
	}
	return out, nil
}

// Index embeds chunks and writes them to st. It is Embed followed by
// IndexEmbedded, so a chunk is embedded from the same text and batched the
// same way, and there is one definition of what gets embedded.
func Index(st Store, emb embed.Embedder, chunks []chunk.Chunk) error {
	if st == nil {
		return fmt.Errorf("store is required")
	}
	embedded, err := Embed(emb, chunks)
	if err != nil {
		return err
	}
	return IndexEmbedded(st, embedded)
}

// IndexEmbedded writes chunks that already carry an Embedding, so a
// caller that ran Embed does not pay for a second batch. Every chunk
// must have a vector.
func IndexEmbedded(st Store, chunks []chunk.Chunk) error {
	if st == nil {
		return fmt.Errorf("store is required")
	}
	if len(chunks) == 0 {
		return nil
	}

	records := make([]Record, len(chunks))
	for i, ch := range chunks {
		if len(ch.Embedding) == 0 {
			return fmt.Errorf("chunk %d has no embedding", i)
		}
		// Drop the vector from the stored copy: Record keeps it in its
		// own field, and duplicating it would double every JSONL line.
		c := ch
		c.Embedding = nil
		records[i] = Record{Chunk: c, Vector: ch.Embedding}
	}
	return st.Upsert(records)
}

func searchRecords(records []Record, query []float64, k int) ([]Hit, error) {
	if k <= 0 {
		return nil, fmt.Errorf("k must be > 0, got %d", k)
	}
	// Cosine scores a dimension mismatch as 0, which would rank every
	// record equally and look like a valid result. That happens when the
	// query is embedded by a different model than the index was, so it is
	// worth an error rather than a silently useless ranking.
	if len(records) > 0 && len(query) != len(records[0].Vector) {
		return nil, fmt.Errorf(
			"query has %d dimensions but the index has %d; the query must use the same embedder as the index",
			len(query), len(records[0].Vector),
		)
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
