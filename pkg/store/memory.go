package store

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
