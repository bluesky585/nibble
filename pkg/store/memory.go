package store

import "sort"

// Memory keeps records in process. It is not safe for concurrent use.
type Memory struct {
	records []Record
}

// Upsert appends records.
func (m *Memory) Upsert(records []Record) error {
	m.records = append(m.records, records...)
	return nil
}

// Search returns the k nearest records by cosine similarity.
func (m *Memory) Search(query []float64, k int) ([]Hit, error) {
	return searchRecords(m.records, query, k)
}

// Records returns the whole corpus, for the ranking modes that read
// texts rather than search vectors. The slice is shared, like the
// JSONL store's: the caller must not modify it.
func (m *Memory) Records() []Record {
	return m.records
}

// Sources counts the records per origin, most records first.
func (m *Memory) Sources() ([]Source, error) {
	return countSources(m.records), nil
}

// DeleteSource drops every record of src. The survivors are compacted
// in place rather than copied to a new slice, so memory returns to the
// pre-index shape after a delete.
func (m *Memory) DeleteSource(src string) (int, error) {
	kept := m.records[:0]
	removed := 0
	for _, rec := range m.records {
		if rec.Source == src {
			removed++
			continue
		}
		kept = append(kept, rec)
	}
	m.records = kept
	return removed, nil
}

// countSources tallies records per source, most records first.
func countSources(records []Record) []Source {
	counts := make(map[string]int)
	for _, rec := range records {
		counts[rec.Source]++
	}
	out := make([]Source, 0, len(counts))
	for name, n := range counts {
		out = append(out, Source{Name: name, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Name < out[j].Name
		}
		return out[i].Count > out[j].Count
	})
	return out
}
