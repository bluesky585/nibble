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
func runQuery(query, path string, k int, emb embed.Embedder, stdout, stderr io.Writer) int {
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

	vecs, err := emb.Embed([]string{query})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if len(vecs) != 1 {
		fmt.Fprintf(stderr, "embedder returned %d vectors for one query\n", len(vecs))
		return 1
	}

	hits, err := st.Search(vecs[0], k)
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
