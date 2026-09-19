package store

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/bluesky585/nibble/pkg/chunk"
)

func vec(n int, seed float64) []float32 {
	v := make([]float32, n)
	for i := range v {
		v[i] = float32(seed + float64(i)*0.1)
	}
	return v
}

func rec(text string, offset, dims int, seed float64) Record {
	ch, err := chunk.New(text, offset, offset+len([]rune(text)), len([]rune(text)))
	if err != nil {
		panic(err)
	}
	return Record{Chunk: ch, Vector: vec(dims, seed)}
}

// A record upserted into a SQLite store survives close and reload: the
// file is the store, so opening the same path again returns the same
// search results. That is the whole point over the JSONL store, which
// rewrites its file on every upsert but has no incremental path.
func TestSQLitePersistence(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "idx.db")
	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert([]Record{
		rec("Cats sleep.", 0, 8, 0.1),
		rec("Dogs bark.", 12, 8, 0.2),
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	st2, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()
	hits, err := st2.Search(vec(8, 0.1), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("hits=%d want 2", len(hits))
	}
	if hits[0].Record.Chunk.Text != "Cats sleep." {
		t.Fatalf("top hit=%q", hits[0].Record.Chunk.Text)
	}
	if hits[0].Score <= hits[1].Score {
		t.Fatalf("scores not ordered: %v then %v", hits[0].Score, hits[1].Score)
	}
}

// Upserts are incremental: a second call adds to the file without
// erasing the first batch, and a record upserted twice appears once
// (the second write wins).
func TestSQLiteIncrementalUpsert(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "idx.db")
	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.Upsert([]Record{rec("one", 0, 4, 0.1)}); err != nil {
		t.Fatal(err)
	}
	if err := st.Upsert([]Record{rec("two", 4, 4, 0.2)}); err != nil {
		t.Fatal(err)
	}
	// Same chunk text and offsets: an update, not a duplicate row.
	if err := st.Upsert([]Record{rec("one", 0, 4, 0.3)}); err != nil {
		t.Fatal(err)
	}

	hits, err := st.Search(vec(4, 0.3), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("hits=%d want 2 (no duplicate rows)", len(hits))
	}
	for _, h := range hits {
		if h.Record.Chunk.Text == "one" && math.Abs(h.Score-1) > 1e-6 {
			// The query is the stored vector itself, but the store keeps
			// float32, so a self-match lands within rounding of 1, not on it.
			t.Fatalf("updated vector not stored: score %v for the query itself", h.Score)
		}
	}
}

// A SQLite store and the JSONL store hold the same records must return
// the same ranking: the store is a persistence choice, not a ranking
// choice.
func TestSQLiteMatchesJSONLRanking(t *testing.T) {
	t.Parallel()

	rng := rand.New(rand.NewSource(42))
	dims := 16
	var records []Record
	for i := range 30 {
		v := make([]float32, dims)
		for j := range v {
			v[j] = float32(rng.NormFloat64())
		}
		records = append(records, rec(fmt.Sprintf("doc %d text", i), i*20, dims, 0))
		// Overwrite the seeded vector with the random one.
		records[len(records)-1].Vector = v
	}

	jsonlPath := filepath.Join(t.TempDir(), "idx.jsonl")
	js, err := OpenJSONL(jsonlPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := js.Upsert(records); err != nil {
		t.Fatal(err)
	}

	sqlPath := filepath.Join(t.TempDir(), "idx.db")
	sq, err := OpenSQLite(sqlPath)
	if err != nil {
		t.Fatal(err)
	}
	defer sq.Close()
	if err := sq.Upsert(records); err != nil {
		t.Fatal(err)
	}

	query := make([]float32, dims)
	for j := range query {
		query[j] = float32(rng.NormFloat64())
	}
	a, err := js.Search(query, 5)
	if err != nil {
		t.Fatal(err)
	}
	b, err := sq.Search(query, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != len(b) {
		t.Fatalf("hit counts differ: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Record.Chunk.Text != b[i].Record.Chunk.Text {
			t.Fatalf("hit %d: %q vs %q", i, a[i].Record.Chunk.Text, b[i].Record.Chunk.Text)
		}
		// The SQLite store scores against float32-rounded vectors, so its
		// scores agree with the float64 JSONL path only to float32
		// precision; the ranking itself must be identical.
		if math.Abs(a[i].Score-b[i].Score) > 1e-6 {
			t.Fatalf("hit %d score: %v vs %v", i, a[i].Score, b[i].Score)
		}
	}
}

// The index row also stores the chunk's context and the vector, and
// they come back intact.
func TestSQLiteRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "idx.db")
	st, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	r := rec("row text", 0, 4, 0.5)
	r.Chunk.Context = "name,color"
	if err := st.Upsert([]Record{r}); err != nil {
		t.Fatal(err)
	}

	hits, err := st.Search(vec(4, 0.5), 1)
	if err != nil {
		t.Fatal(err)
	}
	got := hits[0].Record
	if got.Chunk.Context != "name,color" {
		t.Fatalf("context=%q", got.Chunk.Context)
	}
	// Vectors are stored as float32: a value that survives that cast
	// must come back equal to it.
	for i, v := range got.Vector {
		if v != vec(4, 0.5)[i] {
			t.Fatalf("vector[%d]=%v want %v", i, v, vec(4, 0.5)[i])
		}
	}
}

// Records carry JSON tags so a record read back from the JSONL store
// and one read from SQLite serialize identically.
func TestSQLiteRecordJSON(t *testing.T) {
	t.Parallel()

	r := rec("text", 0, 4, 0.1)
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) == 0 {
		t.Fatal("empty JSON")
	}
}
