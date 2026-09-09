// Package buildchunk constructs a chunker from CLI or API options.
package buildchunk

import (
	"fmt"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/fastchunker"
	"github.com/bluesky585/nibble/pkg/recursive"
	"github.com/bluesky585/nibble/pkg/sentencechunker"
	"github.com/bluesky585/nibble/pkg/tablechunker"
	"github.com/bluesky585/nibble/pkg/tokenchunker"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

// Chunker splits text into chunks.
type Chunker interface {
	Chunk(text string) ([]chunk.Chunk, error)
}

// New builds a chunker. overlap is valid only for the token chunker.
// The tokenizer is ignored when chunkerName is "fast"; size is then a
// byte budget, while chunk offsets remain runes.
func New(chunkerName, tokName string, size, overlap int) (Chunker, error) {
	if chunkerName == "fast" {
		if overlap != 0 {
			return nil, fmt.Errorf("overlap is only supported by the token chunker")
		}
		return fastchunker.New(size, nil)
	}

	tok, err := newTokenizer(tokName)
	if err != nil {
		return nil, err
	}

	switch chunkerName {
	case "recursive":
		if overlap != 0 {
			return nil, fmt.Errorf("overlap is only supported by the token chunker")
		}
		return recursive.New(tok, size, nil)
	case "sentence":
		if overlap != 0 {
			return nil, fmt.Errorf("overlap is only supported by the token chunker")
		}
		return sentencechunker.New(tok, size, nil)
	case "token":
		return tokenchunker.New(tok, size, overlap)
	case "table":
		if overlap != 0 {
			return nil, fmt.Errorf("overlap is only supported by the token chunker")
		}
		return tablechunker.New(tok, size)
	default:
		return nil, fmt.Errorf("unknown chunker %q", chunkerName)
	}
}

func newTokenizer(name string) (tokenizer.Tokenizer, error) {
	switch name {
	case "character":
		return tokenizer.Character{}, nil
	case "word":
		return tokenizer.Word{}, nil
	default:
		return nil, fmt.Errorf("unknown tokenizer %q", name)
	}
}
