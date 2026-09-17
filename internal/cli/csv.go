package cli

import (
	"encoding/csv"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/bluesky585/nibble/pkg/chunk"
)

// CSV and TSV files are tabular input, not prose: the unit of meaning
// is a row, and the column names are retrieval context. Reading one
// yields a chunk per data row with the header row carried in Context,
// matching the table chunker's header-copy semantics. The header itself
// is metadata and never becomes a chunk.

// isTabular reports whether path's extension names a tabular format.
func isTabular(path string) bool {
	_, ok := sepForExt(path)
	return ok
}

// sepForExt names the delimiter a tabular file splits on. The empty
// string means the extension is not tabular and the file reads as plain
// text.
func sepForExt(path string) (rune, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv":
		return ',', true
	case ".tsv":
		return '\t', true
	default:
		return 0, false
	}
}

// readTabular splits r into row chunks. Records are read with
// encoding/csv so quoted fields, embedded delimiters, and embedded
// newlines come through decoded; each chunk's text is the row rejoined
// with the same delimiter, and every chunk carries the header in
// Context. A file with no data rows yields no chunks.
func readTabular(sep rune, r io.Reader) ([]chunk.Chunk, error) {
	cr := csv.NewReader(r)
	cr.Comma = sep
	cr.FieldsPerRecord = -1 // rows may vary in width; be liberal here
	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("tabular: %w", err)
	}
	if len(records) < 2 {
		// Only a header, or nothing: no data rows, no chunks.
		return nil, nil
	}
	header := strings.Join(records[0], string(sep))

	var out []chunk.Chunk
	start := 0
	for _, rec := range records[1:] {
		text := strings.Join(rec, string(sep))
		end := start + len([]rune(text))
		ch, err := chunk.New(text, start, end, end-start)
		if err != nil {
			return nil, err
		}
		ch.Context = header
		out = append(out, ch)
		// One rune between rows for the newline the CSV format holds.
		start = end + 1
	}
	return out, nil
}
