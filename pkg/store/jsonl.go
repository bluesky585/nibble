package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

// JSONL stores records in a JSON Lines file. Each Upsert rewrites the file.
type JSONL struct {
	path    string
	records []Record
}

// OpenJSONL loads existing records from path. A missing file is empty.
func OpenJSONL(path string) (*JSONL, error) {
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}
	s := &JSONL{path: path}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			return nil, err
		}
		s.records = append(s.records, rec)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return s, nil
}

// Upsert appends records and writes the whole file once.
func (s *JSONL) Upsert(records []Record) error {
	s.records = append(s.records, records...)

	f, err := os.Create(s.path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, rec := range s.records {
		if err := enc.Encode(rec); err != nil {
			return err
		}
	}
	return nil
}

// Search returns the k nearest records by cosine similarity.
func (s *JSONL) Search(query []float32, k int) ([]Hit, error) {
	return searchRecords(s.records, query, k)
}

// Records exposes the loaded records for scoring beyond cosine, such as
// a sparse ranker that needs the texts. The slice is shared, not
// copied: callers must not modify it.
func (s *JSONL) Records() []Record {
	return s.records
}

// Sources counts the records per origin, most records first.
func (s *JSONL) Sources() ([]Source, error) {
	return countSources(s.records), nil
}

// DeleteSource drops every record of src and rewrites the file once,
// the way an upsert does. The survivors are compacted in place.
func (s *JSONL) DeleteSource(src string) (int, error) {
	kept := s.records[:0]
	removed := 0
	for _, rec := range s.records {
		if rec.Source == src {
			removed++
			continue
		}
		kept = append(kept, rec)
	}
	s.records = kept
	if removed == 0 {
		return 0, nil
	}

	f, err := os.Create(s.path)
	if err != nil {
		return removed, err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, rec := range s.records {
		if err := enc.Encode(rec); err != nil {
			return removed, err
		}
	}
	return removed, nil
}
