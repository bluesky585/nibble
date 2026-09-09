// Package cli implements the nibble command: read text, chunk it, write JSON.
package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/bluesky585/nibble/internal/buildchunk"
	"github.com/bluesky585/nibble/pkg/chunk"
)

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

	chunkerName := fs.String("chunker", "recursive", "chunker: recursive, sentence, token, fast, or table")
	tokName := fs.String("tokenizer", "character", "tokenizer: character or word (ignored by fast)")
	size := fs.Int("size", 512, "max tokens per chunk (max bytes for fast)")
	overlap := fs.Int("overlap", 0, "token overlap (token chunker only)")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	text, err := readInput(fs.Args(), stdin)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	c, err := buildchunk.New(*chunkerName, *tokName, *size, *overlap)
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
