// Package embed turns text into vectors for semantic splitting.
package embed

// Embedder maps texts to dense vectors. Callers pass a batch; the
// implementation must not create a new client per item.
type Embedder interface {
	Embed(texts []string) ([][]float64, error)
}
