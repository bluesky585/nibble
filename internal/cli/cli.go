// Package cli implements the nibble command: read text, chunk it, write JSON.
package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bluesky585/nibble/internal/buildchunk"
	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/embed"
	olap "github.com/bluesky585/nibble/pkg/overlap"
	"github.com/bluesky585/nibble/pkg/store"
	"github.com/bluesky585/nibble/pkg/tokenizer"
	"github.com/bluesky585/nibble/pkg/visualize"
)

// Run parses args, chunks input, and writes JSON chunks to stdout.
// It returns a process exit code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("nibble", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: nibble [flags] [file]\n")
		fmt.Fprintf(stderr, "  Read UTF-8 text from a file, a directory, or stdin and print JSON.\n\n")
		fs.PrintDefaults()
	}

	chunkerName := fs.String("chunker", "recursive", "chunker: recursive, sentence, token, fast, table, code, markdown, or semantic")
	tokName := fs.String("tokenizer", "character", "tokenizer: character or word (ignored by fast)")
	size := fs.Int("size", 512, "max tokens per chunk (max bytes for fast)")
	overlap := fs.Int("overlap", 0, "token overlap (token chunker only)")
	indexPath := fs.String("index", "", "optional JSONL path to store chunk embeddings")
	embedderName := fs.String("embedder", "hashing", "embedder: hashing or openai (semantic and -index)")
	contextN := fs.Int("context", 0, "neighbor tokens copied into chunk context (0 disables)")
	contextMode := fs.String("context-mode", "prefix", "prefix or suffix (used with -context)")
	dirPath := fs.String("dir", "", "directory to chunk (recursive; not with a file argument)")
	extCSV := fs.String("ext", ".txt,.md", "comma-separated extensions when using -dir")
	htmlPath := fs.String("html", "", "write an HTML page of the source colored by chunk")
	embedInJSON := fs.Bool("embed", false, "add an embedding to each chunk in the JSON output (one batch call)")
	query := fs.String("query", "", "search an -index file for this text and print hits instead of chunking")
	topK := fs.Int("k", 5, "number of hits for -query")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	emb, err := embed.Lookup(*embedderName)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	// -query searches an existing index. It reads no input, so it must
	// not wait on stdin, and it does not need a chunker.
	if *query != "" {
		if *dirPath != "" || len(fs.Args()) > 0 {
			fmt.Fprintln(stderr, "-query reads no input; drop -dir and the file argument")
			return 2
		}
		return runQuery(*query, *indexPath, *topK, emb, stdout, stderr)
	}
	// Catches `nibble -index x.jsonl -k 3` read as a search.
	if *topK != 5 {
		fmt.Fprintln(stderr, "-k is only used with -query")
		return 2
	}

	if *dirPath != "" && len(fs.Args()) > 0 {
		fmt.Fprintln(stderr, "provide either -dir or a file path, not both")
		return 2
	}

	jobs, err := collectJobs(*dirPath, *extCSV, fs.Args(), stdin)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	c, err := buildchunk.New(*chunkerName, *tokName, *size, *overlap, emb)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	var tok tokenizer.Tokenizer
	if *contextN != 0 {
		tok, err = tokenizerFromName(*tokName)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		switch *contextMode {
		case "prefix", "suffix":
		default:
			fmt.Fprintf(stderr, "unknown context-mode %q\n", *contextMode)
			return 2
		}
	}

	var all []chunk.Chunk
	docs := make([]chunk.Document, 0, len(jobs))
	for _, job := range jobs {
		chunks, err := c.Chunk(job.text)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if chunks == nil {
			chunks = []chunk.Chunk{}
		}
		if *contextN != 0 {
			switch *contextMode {
			case "prefix":
				chunks, err = olap.Prefix(chunks, tok, *contextN)
			case "suffix":
				chunks, err = olap.Suffix(chunks, tok, *contextN)
			}
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
		}
		// Embed after overlap so the vector sees the same text that
		// store.Index would embed, including any Context.
		if *embedInJSON {
			chunks, err = store.Embed(emb, chunks)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
		}
		all = append(all, chunks...)
		docs = append(docs, chunk.NewDocument(job.path, job.text, chunks))
	}

	if *indexPath != "" {
		st, err := store.OpenJSONL(*indexPath)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if err := indexChunks(st, emb, all, *embedInJSON); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}

	if *htmlPath != "" {
		if err := writeHTML(*htmlPath, docs); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}

	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	var payload any
	if *dirPath != "" {
		payload = docs
	} else if len(docs) == 1 {
		payload = docs[0].Chunks
	} else {
		payload = []chunk.Chunk{}
	}
	if err := enc.Encode(payload); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// indexChunks writes chunks to st. When the chunks already carry
// vectors from -embed they are reused, so no second batch is sent.
func indexChunks(st store.Store, emb embed.Embedder, chunks []chunk.Chunk, preEmbedded bool) error {
	if preEmbedded {
		return store.IndexEmbedded(st, chunks)
	}
	return store.Index(st, emb, chunks)
}

// writeHTML renders docs to path. The JSON on stdout is unaffected.
func writeHTML(path string, docs []chunk.Document) error {
	pages := make([]visualize.Doc, 0, len(docs))
	for _, d := range docs {
		pages = append(pages, visualize.NewDoc(d.Path, d.Content, d.Chunks))
	}
	out, err := visualize.HTML(pages)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

type inputJob struct {
	path string
	text string
}

func collectJobs(dir, extCSV string, files []string, stdin io.Reader) ([]inputJob, error) {
	if dir != "" {
		rels, err := listDirFiles(dir, extCSV)
		if err != nil {
			return nil, err
		}
		jobs := make([]inputJob, 0, len(rels))
		for _, rel := range rels {
			b, err := os.ReadFile(filepath.Join(dir, rel))
			if err != nil {
				return nil, err
			}
			jobs = append(jobs, inputJob{path: rel, text: string(b)})
		}
		return jobs, nil
	}
	text, err := readInput(files, stdin)
	if err != nil {
		return nil, err
	}
	return []inputJob{{path: "", text: text}}, nil
}

func tokenizerFromName(name string) (tokenizer.Tokenizer, error) {
	switch name {
	case "", "character":
		return tokenizer.Character{}, nil
	case "word":
		return tokenizer.Word{}, nil
	default:
		return nil, fmt.Errorf("unknown tokenizer %q", name)
	}
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
