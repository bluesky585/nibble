package cli

import (
	"path/filepath"
	"strings"

	"github.com/bluesky585/nibble/pkg/store"
)

// isSQLitePath reports whether path names a SQLite index by extension.
func isSQLitePath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".db", ".sqlite", ".sqlite3":
		return true
	}
	return false
}

// openIndex opens the index file at path. The extension picks the
// store: .db/.sqlite/.sqlite3 opens (or creates) a SQLite database,
// anything else the JSONL file. The returned close releases the
// store's handle — a no-op for JSONL, which holds none between writes.
// The caller must have already checked that path is set.
func openIndex(path string) (store.Store, func() error, error) {
	if isSQLitePath(path) {
		sq, err := store.OpenSQLite(path)
		if err != nil {
			return nil, nil, err
		}
		return sq, sq.Close, nil
	}
	js, err := store.OpenJSONL(path)
	if err != nil {
		return nil, nil, err
	}
	return js, func() error { return nil }, nil
}

// indexRecords returns the full record corpus of an open index, for
// the scoring modes that read texts rather than search vectors. The
// store interface cannot expose it (the JSONL shape predates the error
// return), so the two concrete stores are told apart here.
func indexRecords(st store.Store) ([]store.Record, error) {
	switch src := st.(type) {
	case *store.JSONL:
		return src.Records(), nil
	case *store.SQLite:
		return src.Records()
	default:
		return nil, nil
	}
}
