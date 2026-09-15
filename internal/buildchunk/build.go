// Package buildchunk constructs a chunker from CLI or API options.
package buildchunk

import (
	"fmt"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/codechunker"
	"github.com/bluesky585/nibble/pkg/embed"
	"github.com/bluesky585/nibble/pkg/fastchunker"
	"github.com/bluesky585/nibble/pkg/markdownchunker"
	"github.com/bluesky585/nibble/pkg/recursive"
	"github.com/bluesky585/nibble/pkg/semantic"
	"github.com/bluesky585/nibble/pkg/sentencechunker"
	"github.com/bluesky585/nibble/pkg/tablechunker"
	"github.com/bluesky585/nibble/pkg/tokenchunker"
	"github.com/bluesky585/nibble/pkg/tokenizer"
	"github.com/bluesky585/nibble/pkg/tokenizer/tiktoken"
)

// Chunker splits text into chunks.
type Chunker interface {
	Chunk(text string) ([]chunk.Chunk, error)
}

// New builds a chunker. overlap is valid only for the token chunker.
// The tokenizer is ignored when chunkerName is "fast"; size is then a
// byte budget, while chunk offsets remain runes.
// "markdown" routes regions of a document to the code, table, and
// recursive chunkers; the tokenizer applies to its prose and code parts.
// lang names the language for the "code" chunker; empty detects it.
// emb is used by the semantic chunker; nil means embed.Hashing.
func New(chunkerName, tokName, lang string, size, overlap int, emb embed.Embedder) (Chunker, error) {
	if chunkerName == "fast" {
		if overlap != 0 {
			return nil, fmt.Errorf("overlap is only supported by the token chunker")
		}
		return fastchunker.New(size, nil)
	}

	tok, err := NewTokenizer(tokName)
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
	case "code":
		if overlap != 0 {
			return nil, fmt.Errorf("overlap is only supported by the token chunker")
		}
		if lang == "" {
			return codechunker.New(tok, size)
		}
		return codechunker.New(tok, size, codechunker.Language(lang))
	case "markdown":
		if overlap != 0 {
			return nil, fmt.Errorf("overlap is only supported by the token chunker")
		}
		if lang != "" {
			return nil, fmt.Errorf("-lang is only used with -chunker code; markdown reads it from each fence")
		}
		return markdownchunker.New(tok, size)
	case "semantic":
		if overlap != 0 {
			return nil, fmt.Errorf("overlap is only supported by the token chunker")
		}
		if emb == nil {
			emb = embed.Hashing{}
		}
		return semantic.New(tok, emb, size, 0)
	default:
		return nil, fmt.Errorf("unknown chunker %q", chunkerName)
	}
}

// NewTokenizer builds a tokenizer by name. The CLI uses it directly for
// -context, so this is the single tokenizer factory: two of them drifted
// apart once already, and a name known to one was unknown to the other.
func NewTokenizer(name string) (tokenizer.Tokenizer, error) {
	switch name {
	case "", "character":
		return tokenizer.Character{}, nil
	case "word":
		return tokenizer.Word{}, nil
	case "tiktoken":
		return tiktoken.New("")
	default:
		return nil, fmt.Errorf("unknown tokenizer %q", name)
	}
}
