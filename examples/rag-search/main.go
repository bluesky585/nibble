// The retrieval lifecycle in one program: chunk text with a real
// chunker, index it into a SQLite file under per-document sources,
// rank a query with hybrid scoring, narrow a search to one source, and
// delete a source — the pieces the CLI's -index/-query/-source flags
// and the HTTP API's index/search/sources routes are built from. The
// file this writes is an ordinary nibble index: `nibble -list-sources`
// or `nibble -query` can open it after the program exits.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bluesky585/nibble/pkg/embed"
	"github.com/bluesky585/nibble/pkg/recursive"
	"github.com/bluesky585/nibble/pkg/store"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

// corpus is a tiny set of documents, one source name each. Real
// callers would read files or fetch pages; the shape is the same. The
// topics deliberately overlap — navigation appears in birds and dogs,
// smell in dogs and cats — so the rankings have something to separate.
var corpus = map[string]string{
	"cats.md":  "Cats sleep twelve to sixteen hours a day. A cat's sense of smell is far weaker than a dog's.",
	"dogs.md":  "Dogs were domesticated from wolves. A dog's sense of smell is tens of thousands of times keener than a person's.",
	"birds.md": "Birds migrate thousands of miles using the earth's magnetic field to navigate. Some birds sing dialects that differ by region.",
}

func main() {
	dir, err := os.MkdirTemp("", "nibble-rag-example")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "index.db")

	// Chunk with a real chunker: the recursive rule stack (blank line,
	// line, sentence, word) over a rune budget. The same chunker the
	// CLI's default runs.
	c, err := recursive.New(tokenizer.Character{}, 48, nil)
	if err != nil {
		panic(err)
	}

	// Index each document under its source name. The hashing embedder is
	// local bag-of-words similarity — no key, no network — which is all
	// this example needs to show ranking behavior.
	st, err := store.OpenSQLite(path)
	if err != nil {
		panic(err)
	}
	emb := embed.Hashing{}
	for src, text := range corpus {
		chunks, err := c.Chunk(text)
		if err != nil {
			panic(err)
		}
		if err := store.IndexLabeled(st, emb, chunks, src); err != nil {
			panic(err)
		}
	}

	// What does the index hold? Sources list their names and counts,
	// most records first.
	srcs, err := store.Sources(st)
	if err != nil {
		panic(err)
	}
	fmt.Println("sources:")
	for _, s := range srcs {
		fmt.Printf("  %-8s %d chunks\n", s.Name, s.Count)
	}

	// Hybrid search over everything: cosine from the embedded query
	// blended with BM25 term overlap. All three ranking modes read the
	// corpus the same way, so this loop is also what a filtered or dense
	// search looks like with the scoring line changed.
	records, err := store.Records(st)
	if err != nil {
		panic(err)
	}
	hits, err := search(emb, records, "how do animals navigate", 2, store.RankHybrid)
	if err != nil {
		panic(err)
	}
	fmt.Println("\ntop hits for \"how do animals navigate\" (hybrid):")
	for _, h := range hits {
		fmt.Printf("  %.3f [%s] %q\n", h.Score, h.Record.Source, h.Record.Chunk.Text)
	}

	// Narrow a search to one source: filter before scoring, so k counts
	// the filtered corpus. A query about smell over cats.md only finds
	// the cat chunk even though the dog chunk scores higher overall.
	filtered := store.FilterSource(records, "cats.md")
	hits, err = search(emb, filtered, "which animal smells best", 2, store.RankHybrid)
	if err != nil {
		panic(err)
	}
	fmt.Println("\ntop hits for \"which animal smells best\" (source=cats.md):")
	for _, h := range hits {
		fmt.Printf("  %.3f [%s] %q\n", h.Score, h.Record.Source, h.Record.Chunk.Text)
	}

	// Delete a source: exactly one document's chunks leave, and the
	// others keep answering. A re-index of dogs.md would call
	// IndexLabeled again under the same name — upserts replace, not
	// duplicate.
	n, err := store.DeleteSource(st, "dogs.md")
	if err != nil {
		panic(err)
	}
	fmt.Printf("\ndeleted dogs.md: %d chunks\n", n)
	records, err = store.Records(st)
	if err != nil {
		panic(err)
	}
	hits, err = search(emb, records, "which animal smells best", 3, store.RankHybrid)
	if err != nil {
		panic(err)
	}
	fmt.Println("the same query after the delete:")
	for _, h := range hits {
		fmt.Printf("  %.3f [%s] %q\n", h.Score, h.Record.Source, h.Record.Chunk.Text)
	}
	fmt.Printf("\nthe index at %s outlives this process; reopen it with nibble -query\n", path)
}

// search embeds the query, scores every record by the named mode, and
// returns the top k. It is the same shape as the CLI's -query and the
// API's /v1/search, at example size.
func search(emb embed.Embedder, records []store.Record, query string, k int, mode string) ([]store.Hit, error) {
	vecs, err := emb.Embed([]string{query})
	if err != nil {
		return nil, err
	}
	scores, err := store.RankScores(records, query, vecs[0], mode, 0.5)
	if err != nil {
		return nil, err
	}
	return store.Rank(records, scores, k), nil
}
