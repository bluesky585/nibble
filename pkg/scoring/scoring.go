// Package scoring ranks chunks against a query without a vector model.
// BM25 scores term overlap over a fixed corpus; Hybrid blends a dense
// score with a sparse one so both signals shape the ranking.
package scoring

import (
	"fmt"
	"math"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

// TermsFunc splits a text into index terms. It is the sparse side's
// tokenizer, the way embed.Embedder is the dense side's.
type TermsFunc func(text string) []string

// WordTerms splits text into index terms. It delegates to
// tokenizer.Terms, the same segmentation the bigram tokenizer and the
// hashing embedder use: one definition of a term across the sparse
// side, the chunk scale, and the dense side.
func WordTerms(text string) []string {
	return tokenizer.Terms(text)
}

// BM25 is an Okapi BM25 scorer over a corpus fixed at construction. The
// corpus is the whole index: JSONL stores scan linearly, so every record
// is a candidate and the statistics are exact rather than sampled.
type BM25 struct {
	terms TermsFunc
	// docTexts holds the indexed text of each document, because scoring
	// recomputes a document's term counts on demand rather than keeping
	// a term map per document for the scorer's whole life.
	docTexts []string
	// docLens holds each document's term count for length normalization.
	docLens []int
	// docFreq maps a term to the number of documents holding it.
	docFreq map[string]int
	avgLen  float64
}

// BM25 constants. k1 bounds term-frequency saturation; b controls how
// strongly a long document is discounted. These are the usual values.
const (
	k1 = 1.5
	b  = 0.75
)

// retrievalText is the text a chunk is scored on. It holds the same
// content as EmbedText — Context plus Text — but joins the parts with a
// space, because term splitting needs a boundary the vector path does
// not: "name,color" and "apple,red" would otherwise fuse into the
// single term "colorapple".
func retrievalText(ch chunk.Chunk) string {
	if ch.Context == "" {
		return ch.Text
	}
	return ch.Context + " " + ch.Text
}

// NewBM25 builds a scorer over docs. The text scored is the same text
// the dense side would embed — Context included, see retrievalText —
// which keeps one definition of what retrieval sees.
func NewBM25(docs []chunk.Chunk, terms TermsFunc) *BM25 {
	if terms == nil {
		terms = WordTerms
	}
	bm := &BM25{
		terms:    terms,
		docTexts: make([]string, len(docs)),
		docLens:  make([]int, len(docs)),
		docFreq:  make(map[string]int),
	}
	total := 0
	for i, d := range docs {
		text := retrievalText(d)
		bm.docTexts[i] = text
		counts := termCounts(terms(text))
		bm.docLens[i] = len(counts)
		total += len(counts)
		// One document counts once per term, however many times the
		// term repeats inside it; counting occurrences would push df
		// toward N and make a repeated term look ubiquitous.
		for t := range counts {
			bm.docFreq[t]++
		}
	}
	if len(docs) > 0 {
		bm.avgLen = float64(total) / float64(len(docs))
	}
	return bm
}

// Score returns the BM25 weight of document i against the query text.
// An out-of-range index, a term absent from the corpus, a term present
// in every document, and an empty query all contribute 0.
func (bm *BM25) Score(i int, query string) float64 {
	if i < 0 || i >= len(bm.docLens) || len(bm.docLens) == 0 {
		return 0
	}
	queryCounts := termCounts(bm.terms(query))
	if len(queryCounts) == 0 {
		return 0
	}
	docCounts := termCounts(bm.terms(bm.docTexts[i]))

	n := float64(len(bm.docLens))
	dl := float64(bm.docLens[i])
	score := 0.0
	for term, qtf := range queryCounts {
		if qtf <= 0 {
			continue
		}
		df := bm.docFreq[term]
		if df == 0 {
			// Absent from the corpus: no evidence anywhere.
			continue
		}
		// The Lucene form of idf, with a +1 inside the log: the plain
		// Okapi formula goes to 0 (or negative) on small corpora, where
		// N-df+0.5 can land at or below df+0.5, and half of a two-document
		// corpus is the normal case for an index being built. The +1 keeps
		// a term's weight positive at every df. A term in every document
		// still scores, but its weight is the smallest the formula gives:
		// log(1 + 0.5/(N+0.5)), which shrinks as the corpus grows. The
		// ubiquitous term is therefore not special-cased — with N=1 every
		// term is ubiquitous, and short-circuiting it would make a
		// one-document index score every query at 0.
		idf := math.Log(1 + (n-float64(df)+0.5)/(float64(df)+0.5))
		tf := float64(docCounts[term])
		if tf == 0 {
			continue
		}
		norm := tf * (k1 + 1) / (tf + k1*(1-b+b*dl/bm.avgLen))
		score += idf * norm
	}
	return score
}

// termCounts counts occurrences per term, preserving no order.
func termCounts(terms []string) map[string]int {
	counts := make(map[string]int, len(terms))
	for _, t := range terms {
		counts[t]++
	}
	return counts
}

// Hybrid blends a dense score with a sparse one. The dense side is the
// caller's ranking (cosine over a query vector); the sparse side is a
// BM25 built over the same corpus. weight is the dense share: 1 is the
// dense ranking alone, 0 the sparse one.
type Hybrid struct {
	dense  []float64
	sparse *BM25
	weight float64
}

// NewHybrid builds a scorer. The weight must be in [0, 1].
func NewHybrid(dense []float64, sparse *BM25, weight float64) (*Hybrid, error) {
	if weight < 0 || weight > 1 {
		return nil, errWeight{weight}
	}
	return &Hybrid{dense: dense, sparse: sparse, weight: weight}, nil
}

// errWeight reports an out-of-range blend weight.
type errWeight struct{ got float64 }

func (e errWeight) Error() string {
	return fmt.Sprintf("hybrid weight must be in [0, 1], got %v", e.got)
}

// Score returns the blended score of document i. An out-of-range index
// scores 0.
func (h *Hybrid) Score(i int, query string) float64 {
	d := 0.0
	if i >= 0 && i < len(h.dense) {
		d = h.dense[i]
	}
	s := h.sparse.Score(i, query)
	return h.weight*d + (1-h.weight)*s
}
