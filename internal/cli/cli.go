// Package cli implements the nibble command: read text, chunk it, write JSON.
package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/recursive"
	"github.com/bluesky585/nibble/pkg/sentencechunker"
	"github.com/bluesky585/nibble/pkg/tokenchunker"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

type chunker interface {
	Chunk(text string) ([]chunk.Chunk, error)
}

// Run parses args, chunks input, and writes JSON chunks to stdout.
// It returns a process exit code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("nibble", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: nibble [flags] [file]\n")
		fmt.Fprintf(stderr, "  Read a UTF-8 text file (or stdin) and print chunks as JSON.\n\n")
		fs.PrintDefaults()
	}

	chunkerName := fs.String("chunker", "recursive", "chunker: recursive, sentence, or token")
	tokName := fs.String("tokenizer", "character", "tokenizer: character or word")
	size := fs.Int("size", 512, "max tokens per chunk")
	overlap := fs.Int("overlap", 0, "token overlap (token chunker only)")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	text, err := readInput(fs.Args(), stdin)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	c, err := newChunker(*chunkerName, *tokName, *size, *overlap)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	chunks, err := c.Chunk(text)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if chunks == nil {
		chunks = []chunk.Chunk{}
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(chunks); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func readInput(files []string, stdin io.Reader) (string, error) {
	switch len(files) {
	case 0:
		b, err := io.ReadAll(stdin)
		if err != nil {
			return "", err
		}
		return string(b), nil
	case 1:
		b, err := os.ReadFile(files[0])
		if err != nil {
			return "", err
		}
		return string(b), nil
	default:
		return "", fmt.Errorf("expected at most one file, got %s", strings.Join(files, ", "))
	}
}

func newChunker(chunkerName, tokName string, size, overlap int) (chunker, error) {
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
