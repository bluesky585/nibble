// Package assertchunk checks split invariants used by chunker tests.
package assertchunk

import (
	"fmt"
	"strings"
	"testing"

	"github.com/bluesky585/nibble/pkg/chunk"
)

// Check reports whether chunks are a complete, non-overlapping split of original.
//
// The expected invariants:
//   - the first chunk starts at rune 0
//   - adjacent chunks meet (previous End equals next Start)
//   - the last chunk ends at the rune count of original
//   - each chunk.Text equals original[Start:End] in rune space
//   - concatenating chunk texts equals original
func Check(original string, chunks []chunk.Chunk) error {
	runes := []rune(original)

	if len(chunks) == 0 {
		if original != "" {
			return fmt.Errorf("empty chunk list does not cover %d-rune original", len(runes))
		}
		return nil
	}

	if chunks[0].Start != 0 {
		return fmt.Errorf("first chunk start is %d, want 0", chunks[0].Start)
	}
	if last := chunks[len(chunks)-1]; last.End != len(runes) {
		return fmt.Errorf("last chunk end is %d, want %d", last.End, len(runes))
	}

	var b strings.Builder
	for i, c := range chunks {
		if i > 0 {
			prev := chunks[i-1]
			if prev.End != c.Start {
				return fmt.Errorf(
					"gap or overlap between chunk %d end %d and chunk %d start %d",
					i-1, prev.End, i, c.Start,
				)
			}
		}
		if c.Start < 0 || c.End > len(runes) {
			return fmt.Errorf(
				"chunk %d range [%d, %d) is outside original of %d runes",
				i, c.Start, c.End, len(runes),
			)
		}
		got := string(runes[c.Start:c.End])
		if got != c.Text {
			return fmt.Errorf("chunk %d text does not match original[%d:%d]", i, c.Start, c.End)
		}
		b.WriteString(c.Text)
	}

	if b.String() != original {
		return fmt.Errorf("concatenated chunks do not equal original")
	}
	return nil
}

// Split fails the test if chunks are not a complete, non-overlapping split of original.
func Split(t testing.TB, original string, chunks []chunk.Chunk) {
	t.Helper()
	if err := Check(original, chunks); err != nil {
		t.Fatal(err)
	}
}
