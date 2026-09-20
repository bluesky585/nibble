package embed

import (
	"hash/fnv"

	"github.com/bluesky585/nibble/pkg/tokenizer"
)

const hashingDim = 64

// Hashing embeds text with a bag-of-words hashing trick.
// Same terms land in the same slots, so overlapping wording is similar.
// It needs no network and is deterministic.
type Hashing struct{}

// Embed returns one hashingDim vector per input. An empty string is zeros.
//
// Terms come from tokenizer.Terms — the same segmentation the bigram
// tokenizer and the sparse side use — rather than whitespace fields.
// For Latin text the two agree closely enough that ranking is usually
// unchanged, but the vectors are not identical, and CJK text changes
// fundamentally: a whitespace split turned a whole sentence into one
// hashed slot, where bigrams give retrieval its unit. An index built
// before this change must be rebuilt.
func (Hashing) Embed(texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	h := fnv.New32a()
	for i, text := range texts {
		v := make([]float32, hashingDim)
		for _, term := range tokenizer.Terms(text) {
			h.Reset()
			_, _ = h.Write([]byte(term))
			v[int(h.Sum32())%hashingDim] += 1
		}
		out[i] = v
	}
	return out, nil
}
