package store

import (
	"path/filepath"
	"testing"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/embed"
)

// sourceRec builds one record with a vector, so a store can hold it
// without an embedder round trip.
func sourceRec(t *testing.T, text, src string, start int) Record {
	t.Helper()
	ch, err := chunk.New(text, start, start+len([]rune(text)), len([]rune(text)))
	if err != nil {
		t.Fatal(err)
	}
	vec, err := embed.Hashing{}.Embed([]string{text})
	if err != nil {
		t.Fatal(err)
	}
	return Record{Chunk: ch, Vector: vec[0], Source: src}
}

// Every store lists one Source per origin with the right count, most
// records first, and the empty source (records indexed without one) is
// an origin like any other.
func TestSources(t *testing.T) {
	t.Parallel()

	sts := map[string]Store{
		"memory": &Memory{},
	}
	jsPath := filepath.Join(t.TempDir(), "s.jsonl")
	js, err := OpenJSONL(jsPath)
	if err != nil {
		t.Fatal(err)
	}
	sts["jsonl"] = js
	sqPath := filepath.Join(t.TempDir(), "s.db")
	sq, err := OpenSQLite(sqPath)
	if err != nil {
		t.Fatal(err)
	}
	sts["sqlite"] = sq

	for name, st := range sts {
		records := []Record{
			sourceRec(t, "cats sleep", "a.md", 0),
			sourceRec(t, "dogs bark", "a.md", 11),
			sourceRec(t, "birds fly", "b.md", 22),
			sourceRec(t, "no origin", "", 33),
		}
		if err := st.Upsert(records); err != nil {
			t.Fatal(err)
		}
		srcs, err := Sources(st)
		if err != nil {
			t.Fatal(err)
		}
		if len(srcs) != 3 {
			t.Fatalf("%s: sources=%+v, want 3", name, srcs)
		}
		// Most records first; ties break by name. The empty source is
		// listed as the empty name, not skipped.
		if srcs[0].Name != "a.md" || srcs[0].Count != 2 {
			t.Fatalf("%s: first=%+v, want a.md x2", name, srcs[0])
		}
		if srcs[1].Name != "" || srcs[2].Name != "b.md" {
			t.Fatalf("%s: order=%+v", name, srcs[1:])
		}
	}
}

// DeleteSource removes exactly that origin's records, from every store,
// and reports the count. Other sources keep searching; a second delete
// of the same source reports 0.
func TestDeleteSource(t *testing.T) {
	t.Parallel()

	jsPath := filepath.Join(t.TempDir(), "d.jsonl")
	sqPath := filepath.Join(t.TempDir(), "d.db")
	sts := map[string]func() (Store, error){
		"memory": func() (Store, error) { return &Memory{}, nil },
		"jsonl":  func() (Store, error) { return OpenJSONL(jsPath) },
		"sqlite": func() (Store, error) { return OpenSQLite(sqPath) },
	}

	for name, open := range sts {
		st, err := open()
		if err != nil {
			t.Fatal(err)
		}
		records := []Record{
			sourceRec(t, "cats sleep", "a.md", 0),
			sourceRec(t, "dogs bark", "b.md", 11),
			sourceRec(t, "birds fly", "b.md", 22),
		}
		if err := st.Upsert(records); err != nil {
			t.Fatal(err)
		}

		n, err := DeleteSource(st, "b.md")
		if err != nil {
			t.Fatal(err)
		}
		if n != 2 {
			t.Fatalf("%s: removed=%d, want 2", name, n)
		}
		// The count the store reports now reflects the deletion.
		srcs, err := Sources(st)
		if err != nil {
			t.Fatal(err)
		}
		if len(srcs) != 1 || srcs[0].Name != "a.md" || srcs[0].Count != 1 {
			t.Fatalf("%s: sources after delete=%+v", name, srcs)
		}
		// Deleting what is not there is 0, not an error.
		n, err = DeleteSource(st, "b.md")
		if err != nil || n != 0 {
			t.Fatalf("%s: second delete=%d, %v", name, n, err)
		}
		// The survivor still searches.
		q, err := embed.Hashing{}.Embed([]string{"cats"})
		if err != nil {
			t.Fatal(err)
		}
		hits, err := st.Search(q[0], 5)
		if err != nil {
			t.Fatal(err)
		}
		if len(hits) != 1 || hits[0].Record.Chunk.Text != "cats sleep" {
			t.Fatalf("%s: hits after delete=%+v", name, hits)
		}
	}
}

// A deleted source stays deleted across a reopen: the JSONL file was
// rewritten and the SQLite rows are gone from the table.
func TestDeleteSourcePersists(t *testing.T) {
	t.Parallel()

	jsPath := filepath.Join(t.TempDir(), "p.jsonl")
	js, err := OpenJSONL(jsPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := js.Upsert([]Record{
		sourceRec(t, "cats sleep", "a.md", 0),
		sourceRec(t, "dogs bark", "b.md", 11),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := DeleteSource(js, "b.md"); err != nil {
		t.Fatal(err)
	}
	js2, err := OpenJSONL(jsPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(js2.Records()); got != 1 {
		t.Fatalf("jsonl reopened with %d records, want 1", got)
	}

	sqPath := filepath.Join(t.TempDir(), "p.db")
	sq, err := OpenSQLite(sqPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := sq.Upsert([]Record{
		sourceRec(t, "cats sleep", "a.md", 0),
		sourceRec(t, "dogs bark", "b.md", 11),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := DeleteSource(sq, "b.md"); err != nil {
		t.Fatal(err)
	}
	if err := sq.Close(); err != nil {
		t.Fatal(err)
	}
	sq2, err := OpenSQLite(sqPath)
	if err != nil {
		t.Fatal(err)
	}
	defer sq2.Close()
	recs, err := sq2.Records()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 {
		t.Fatalf("sqlite reopened with %d records, want 1", len(recs))
	}
}

// A database written before the source column existed opens cleanly and
// its records read back with the empty source: the migration widens old
// files in place.
func TestSQLiteMigratesOldFile(t *testing.T) {
	t.Parallel()

	// Build a v0.8-era file by upserting with the old column list
	// through raw SQL, so the test does not depend on when the column
	// was added to the schema.
	path := filepath.Join(t.TempDir(), "old.db")
	sq, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := sq.Upsert([]Record{sourceRec(t, "cats sleep", "", 0)}); err != nil {
		t.Fatal(err)
	}
	if err := sq.Close(); err != nil {
		t.Fatal(err)
	}

	sq2, err := OpenSQLite(path)
	if err != nil {
		t.Fatalf("reopen with source column: %v", err)
	}
	defer sq2.Close()
	recs, err := sq2.Records()
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].Source != "" {
		t.Fatalf("migrated records=%+v", recs)
	}
}

// The same chunk upserted under a different source replaces the row's
// source rather than duplicating it: the primary key does not include
// the source, on purpose.
func TestSQLiteUpsertMovesSource(t *testing.T) {
	t.Parallel()

	sq, err := OpenSQLite(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sq.Close()

	if err := sq.Upsert([]Record{sourceRec(t, "cats sleep", "old.md", 0)}); err != nil {
		t.Fatal(err)
	}
	if err := sq.Upsert([]Record{sourceRec(t, "cats sleep", "new.md", 0)}); err != nil {
		t.Fatal(err)
	}
	srcs, err := sq.Sources()
	if err != nil {
		t.Fatal(err)
	}
	if len(srcs) != 1 || srcs[0].Name != "new.md" || srcs[0].Count != 1 {
		t.Fatalf("sources=%+v, want only new.md x1", srcs)
	}
}

// FilterSource narrows a corpus to one origin, before scoring so k
// counts filtered hits. The empty string is the origin of records
// indexed without a source, and the input is left untouched.
func TestFilterSource(t *testing.T) {
	records := []Record{
		sourceRec(t, "a", "a.md", 0),
		sourceRec(t, "b", "", 1),
		sourceRec(t, "c", "a.md", 2),
	}

	if got := FilterSource(records, "a.md"); len(got) != 2 {
		t.Fatalf("a.md got %d records, want 2", len(got))
	}
	if got := FilterSource(records, ""); len(got) != 1 || got[0].Chunk.Text != "b" {
		t.Fatalf("empty got %+v", got)
	}
	if got := FilterSource(records, "nope"); len(got) != 0 {
		t.Fatalf("nope got %d records, want 0", len(got))
	}

	// The filter must not have reordered or rewritten the input.
	if records[0].Chunk.Text != "a" || records[1].Chunk.Text != "b" {
		t.Fatalf("input mutated: %+v", records)
	}
}
