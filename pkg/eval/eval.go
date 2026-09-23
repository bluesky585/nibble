// Package eval measures retrieval quality against a fixed golden set.
//
// The chunkers and scorers have benchmarks for speed and round-trip
// tests for structure, but nothing answered "does a search return the
// right chunk". The golden set below fixes that: documents on distinct
// topics, queries that name those topics, and the chunk of each
// document the query is about. A test runs the full retrieval path —
// chunk, embed, index, score, rank — and fails when recall or rank
// drops below the bar, so a change to the embedding, the term
// splitter, or a scoring mode cannot silently degrade retrieval.
package eval

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/embed"
	"github.com/bluesky585/nibble/pkg/store"
)

// A Case is one query and the chunk that answers it. Relevant names
// the document the answer lives in; Answers are fragments of that
// chunk's text, matched after whitespace normalization so the exact
// cut the chunker chooses does not have to be guessed here.
type Case struct {
	Query    string
	Relevant string   // source name of the document holding the answer
	Answers  []string // fragments of the answering chunk's text
}

// A Set is a golden set: the documents to index and the cases to score.
type Set struct {
	// Docs maps a source name to its text. One document per topic is
	// enough — a relevant chunk must beat every other document's, which
	// is the discrimination retrieval is asked for.
	Docs  map[string]string
	Cases []Case
}

// Chunker is what Run needs of a chunker: the one method the whole
// package family shares. Any chunker in this repository satisfies it.
type Chunker interface {
	Chunk(text string) ([]chunk.Chunk, error)
}

// A Result is one case's outcome.
type Result struct {
	Case   Case
	Hit    bool    // a relevant chunk appeared in the top k
	Rank   int     // 1-based rank of the first relevant hit; 0 when none
	Score  float64 // score of that hit; 0 when none
	TopHit string  // text of whatever ranked first, for failure messages
}

// RecallAtK is the share of cases with a relevant chunk in the top k.
func RecallAtK(results []Result) float64 {
	if len(results) == 0 {
		return 0
	}
	hit := 0
	for _, r := range results {
		if r.Hit {
			hit++
		}
	}
	return float64(hit) / float64(len(results))
}

// MRR is the mean of the reciprocal ranks: 1.0 when every query's
// answer ranks first, 0.5 when it ranks second, 0 when it is missed.
func MRR(results []Result) float64 {
	if len(results) == 0 {
		return 0
	}
	sum := 0.0
	for _, r := range results {
		if r.Hit {
			sum += 1.0 / float64(r.Rank)
		}
	}
	return sum / float64(len(results))
}

// Run indexes the set's documents and scores every case, using the
// same retrieval path the CLI and API run: the chunker splits each
// document, IndexLabeled embeds and stores it under its source name,
// and RankScores with the named mode ranks the query. k is how deep
// the ranking is searched for a relevant chunk.
func Run(c Chunker, docs map[string]string, cases []Case, emb embed.Embedder, mode string, weight float64, k int) ([]Result, error) {
	st := &store.Memory{}
	// Indexing walks the documents by name, not by map order: equal
	// scores tie on chunk start (Rank's rule), chunks of different
	// documents share starts, and a random walk would make the ranking
	// — and this package's verdicts — flip from run to run.
	names := make([]string, 0, len(docs))
	for src := range docs {
		names = append(names, src)
	}
	sort.Strings(names)
	for _, src := range names {
		chunks, err := c.Chunk(docs[src])
		if err != nil {
			return nil, fmt.Errorf("chunk %s: %w", src, err)
		}
		if err := store.IndexLabeled(st, emb, chunks, src); err != nil {
			return nil, fmt.Errorf("index %s: %w", src, err)
		}
	}
	records, err := store.Records(st)
	if err != nil {
		return nil, err
	}

	vecs, err := emb.Embed(queryTexts(cases))
	if err != nil {
		return nil, err
	}
	results := make([]Result, len(cases))
	for i, c := range cases {
		scores, err := store.RankScores(records, c.Query, vecs[i], mode, weight)
		if err != nil {
			return nil, err
		}
		results[i] = judge(c, store.Rank(records, scores, k))
	}
	return results, nil
}

// judge inspects the ranking for the first hit whose text contains
// one of the case's answers, from the document the case names.
func judge(c Case, hits []store.Hit) Result {
	res := Result{Case: c}
	if len(hits) > 0 {
		res.TopHit = hits[0].Record.Chunk.Text
	}
	for i, h := range hits {
		if h.Record.Source != c.Relevant || !matchAnswer(h.Record.Chunk.Text, c.Answers) {
			continue
		}
		res.Hit = true
		res.Rank = i + 1
		res.Score = h.Score
		break
	}
	return res
}

// matchAnswer reports whether text contains any answer, after
// collapsing whitespace runs: the chunker owns where a cut falls, so
// an answer lives somewhere inside its chunk, not at the front — a
// query about the middle of a paragraph lands on a chunk whose text
// begins a sentence earlier or later than the answer.
func matchAnswer(text string, answers []string) bool {
	norm := normalize(text)
	for _, a := range answers {
		if strings.Contains(norm, normalize(a)) {
			return true
		}
	}
	return false
}

func normalize(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func queryTexts(cases []Case) []string {
	qs := make([]string, len(cases))
	for i, c := range cases {
		qs[i] = c.Query
	}
	return qs
}
