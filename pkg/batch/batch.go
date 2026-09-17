// Package batch chunks many texts through one chunker.
package batch

import (
	"fmt"

	"github.com/bluesky585/nibble/pkg/chunk"
)

// Chunker is what Chunk batches over. Every chunker package's Chunker
// satisfies it; so does buildchunk.Chunker, which is how the CLI and the
// HTTP API call this.
type Chunker interface {
	Chunk(text string) ([]chunk.Chunk, error)
}

// Chunk chunks each text with c and reports one result per input. The
// results line up with the inputs by index, and nil results become empty
// slices, so a caller that stores results by position never sees a nil it
// did not ask for.
//
// The first error stops the run and is wrapped with the input's index,
// so a caller with many inputs learns which one failed, not just that
// something did. Results before the failing input are discarded: the
// error path returns no partial batch, because a caller that receives an
// error cannot tell which results are trustworthy without re-deriving
// the cutoff itself.
func Chunk(c Chunker, texts []string) ([][]chunk.Chunk, error) {
	out := make([][]chunk.Chunk, len(texts))
	for i, text := range texts {
		chunks, err := c.Chunk(text)
		if err != nil {
			return nil, fmt.Errorf("input %d: %w", i, err)
		}
		if chunks == nil {
			chunks = []chunk.Chunk{}
		}
		out[i] = chunks
	}
	return out, nil
}
