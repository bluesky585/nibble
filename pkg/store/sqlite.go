package store

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, registered under "sqlite"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/embed"
)

// SQLite stores records in a SQLite database file: one table, one row
// per record. It is the JSONL store's persistence with none of its
// costs — an upsert writes the rows it touched rather than the whole
// file, a reopen reads the file instead of the process, and reads after
// that are the same brute-force cosine the JSONL store does, exact
// rather than approximate. At the scales where a linear scan is too
// slow, the store interface is where an ANN backend would slot in; this
// package does not grow one.
//
// Vectors are stored as float32. A float64 embedding rounded to float32
// keeps its ranking in practice: the quantization error is orders of
// magnitude below the gaps between similarity scores, and the file
// halves in size. The score returned is computed from the stored
// float32 vector, so a round trip reports the score of what it kept.
//
// The driver is imported for its registration only; every use here goes
// through database/sql.

// SQLite is a store backed by a SQLite database file. It is not safe
// for concurrent use, like Memory.
type SQLite struct {
	db *sql.DB
}

// sqliteSchema is one table. The row is keyed by the chunk's text and
// offsets, so an upserted chunk updates rather than duplicates; the
// vector is a float32 blob; the rest of the chunk are columns so search
// results are intact chunks.
var sqliteSchema = `
CREATE TABLE IF NOT EXISTS records (
	text        TEXT NOT NULL,
	start       INTEGER NOT NULL,
	end         INTEGER NOT NULL,
	context     TEXT NOT NULL DEFAULT '',
	token_count INTEGER NOT NULL,
	vector      BLOB NOT NULL,
	PRIMARY KEY (text, start, end)
);
`

const sqliteUpsert = `
INSERT INTO records (text, start, end, context, token_count, source, vector)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (text, start, end) DO UPDATE SET
	context = excluded.context,
	token_count = excluded.token_count,
	source = excluded.source,
	vector = excluded.vector;
`

// OpenSQLite opens (or creates) the database at path and ensures the
// schema exists. A missing file is created; an existing file keeps its
// records. A file written before the source column existed is widened
// in place: every row it holds gets the empty source, which is what
// those records mean.
func OpenSQLite(path string) (*SQLite, error) {
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("sqlite: %w", err)
	}
	// The store runs one statement at a time and never concurrent with
	// itself; one connection avoids SQLITE_BUSY between handles.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(sqliteSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite: schema: %w", err)
	}
	// The primary key carries no source, so the same chunk upserted
	// from a different source updates the row's source rather than
	// duplicating it — a re-index under a new name replaces, not adds.
	if _, err := db.Exec(`ALTER TABLE records ADD COLUMN source TEXT NOT NULL DEFAULT ''`); err != nil {
		// A duplicate-column error means the file already has the column,
		// which is the normal reopen path; anything else is real.
		if !strings.Contains(err.Error(), "duplicate column") {
			db.Close()
			return nil, fmt.Errorf("sqlite: migrate source column: %w", err)
		}
	}
	return &SQLite{db: db}, nil
}

// Upsert writes records. Rows are keyed by (text, start, end): the same
// chunk upserted twice updates its context and vector rather than
// duplicating.
func (s *SQLite) Upsert(records []Record) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store is closed")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("sqlite: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(sqliteUpsert)
	if err != nil {
		return fmt.Errorf("sqlite: %w", err)
	}
	defer stmt.Close()

	for i, rec := range records {
		blob, err := encodeVector(rec.Vector)
		if err != nil {
			return fmt.Errorf("record %d: %w", i, err)
		}
		if _, err := stmt.Exec(rec.Chunk.Text, rec.Chunk.Start, rec.Chunk.End,
			rec.Chunk.Context, rec.Chunk.TokenCount, rec.Source, blob); err != nil {
			return fmt.Errorf("record %d: sqlite: %w", i, err)
		}
	}
	return tx.Commit()
}

// Sources counts the records per origin, most records first, straight
// from the column.
func (s *SQLite) Sources() ([]Source, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store is closed")
	}
	rows, err := s.db.Query(`SELECT source, COUNT(*) FROM records GROUP BY source`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: %w", err)
	}
	defer rows.Close()

	var out []Source
	for rows.Next() {
		var src Source
		if err := rows.Scan(&src.Name, &src.Count); err != nil {
			return nil, fmt.Errorf("sqlite: %w", err)
		}
		out = append(out, src)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: %w", err)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Name < out[j].Name
		}
		return out[i].Count > out[j].Count
	})
	return out, nil
}

// DeleteSource removes every record of src and reports the count. The
// affected rows go in one statement.
func (s *SQLite) DeleteSource(src string) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("store is closed")
	}
	res, err := s.db.Exec(`DELETE FROM records WHERE source = ?`, src)
	if err != nil {
		return 0, fmt.Errorf("sqlite: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("sqlite: rows affected: %w", err)
	}
	return int(n), nil
}

// Search returns the k nearest records by cosine similarity, computed
// against the stored float32 vectors.
func (s *SQLite) Search(query []float64, k int) ([]Hit, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store is closed")
	}
	if k <= 0 {
		return nil, fmt.Errorf("k must be > 0, got %d", k)
	}
	rows, err := s.db.Query(`SELECT text, start, end, context, token_count, source, vector FROM records`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: %w", err)
	}
	defer rows.Close()

	var hits []Hit
	for rows.Next() {
		var text, context, source string
		var start, end, tokenCount int
		var blob []byte
		if err := rows.Scan(&text, &start, &end, &context, &tokenCount, &source, &blob); err != nil {
			return nil, fmt.Errorf("sqlite: %w", err)
		}
		vec, err := decodeVector(blob)
		if err != nil {
			return nil, err
		}
		ch, err := chunk.New(text, start, end, tokenCount)
		if err != nil {
			return nil, err
		}
		ch.Context = context
		hits = append(hits, Hit{
			Record: Record{Chunk: ch, Vector: vec, Source: source},
			Score:  embed.Cosine(query, vec),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: %w", err)
	}

	// The same ordering the shared cosine search applies: score
	// descending, ties broken by chunk start.
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

// Records returns every record in the file, ordered by chunk start.
// It backs query-time scoring modes (term overlap) that read the whole
// corpus rather than search vectors. The slice is freshly built, so the
// caller may keep or modify it.
func (s *SQLite) Records() ([]Record, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("store is closed")
	}
	rows, err := s.db.Query(`SELECT text, start, end, context, token_count, source, vector FROM records ORDER BY start`)
	if err != nil {
		return nil, fmt.Errorf("sqlite: %w", err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var text, context, source string
		var start, end, tokenCount int
		var blob []byte
		if err := rows.Scan(&text, &start, &end, &context, &tokenCount, &source, &blob); err != nil {
			return nil, fmt.Errorf("sqlite: %w", err)
		}
		vec, err := decodeVector(blob)
		if err != nil {
			return nil, err
		}
		ch, err := chunk.New(text, start, end, tokenCount)
		if err != nil {
			return nil, err
		}
		ch.Context = context
		records = append(records, Record{Chunk: ch, Vector: vec, Source: source})
	}
	return records, rows.Err()
}

// Close releases the database handle. The file remains; reopening the
// path resumes from it.
func (s *SQLite) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

// encodeVector packs a float64 vector as little-endian float32 bytes.
func encodeVector(v []float64) ([]byte, error) {
	if len(v) == 0 {
		return nil, fmt.Errorf("empty vector")
	}
	out := make([]byte, 4*len(v))
	for i, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return nil, fmt.Errorf("vector[%d] is not finite", i)
		}
		u := math.Float32bits(float32(x))
		out[4*i] = byte(u)
		out[4*i+1] = byte(u >> 8)
		out[4*i+2] = byte(u >> 16)
		out[4*i+3] = byte(u >> 24)
	}
	return out, nil
}

// decodeVector unpacks what encodeVector wrote.
func decodeVector(b []byte) ([]float64, error) {
	if len(b)%4 != 0 {
		return nil, fmt.Errorf("sqlite: vector blob is %d bytes, not a multiple of 4", len(b))
	}
	out := make([]float64, len(b)/4)
	for i := range out {
		u := uint32(b[4*i]) | uint32(b[4*i+1])<<8 | uint32(b[4*i+2])<<16 | uint32(b[4*i+3])<<24
		out[i] = float64(math.Float32frombits(u))
	}
	return out, nil
}
