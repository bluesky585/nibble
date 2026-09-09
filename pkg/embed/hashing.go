package embed

import (
	"hash/fnv"
	"strings"
)

const hashingDim = 64

// Hashing embeds text with a bag-of-words hashing trick.
// Same words land in the same slots, so overlapping wording is similar.
// It needs no network and is deterministic.
type Hashing struct{}

// Embed returns one hashingDim vector per input. An empty string is zeros.
func (Hashing) Embed(texts []string) ([][]float64, error) {
	out := make([][]float64, len(texts))
	h := fnv.New32a()
	for i, text := range texts {
		v := make([]float64, hashingDim)
		for _, word := range strings.Fields(strings.ToLower(text)) {
			h.Reset()
			_, _ = h.Write([]byte(word))
			v[int(h.Sum32())%hashingDim] += 1
		}
		out[i] = v
	}
	return out, nil
}
