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
	"github.com/bluesky585/nibble/pkg/batch"
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
	tokName := fs.String("tokenizer", "character", "tokenizer: character, word, or tiktoken (ignored by fast)")
	lang := fs.String("lang", "", "language for -chunker code: go or python (empty detects it)")
	rulesPath := fs.String("rules", "", "JSON file with a rule hierarchy for -chunker recursive (empty uses the defaults)")
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
	scoringName := fs.String("scoring", "dense", "scoring for -query: dense, bm25, or hybrid")
	hybridWeight := fs.Float64("hybrid-weight", 0.5, "dense share when -scoring hybrid (0 to 1)")
	// Source management: a source names where an indexed chunk came
	// from, so a re-indexed document can replace its old chunks without
	// rebuilding the whole index.
	sourceName := fs.String("source", "", "name the origin of the indexed chunks (with -index); an upserted chunk under a new source moves there")
	listSources := fs.Bool("list-sources", false, "print the sources an -index holds as JSON and exit")
	deleteSource := fs.String("delete-source", "", "remove every chunk of this source from the -index and exit")

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
		return runQuery(*query, *indexPath, *scoringName, *hybridWeight, *topK, emb, stdout, stderr)
	}
	// Catches `nibble -index x.jsonl -k 3` read as a search.
	if *topK != 5 {
		fmt.Fprintln(stderr, "-k is only used with -query")
		return 2
	}

	// -list-sources and -delete-source manage an existing index. Like
	// -query they read no input, so neither may wait on stdin.
	if *listSources || *deleteSource != "" {
		if *dirPath != "" || len(fs.Args()) > 0 {
			fmt.Fprintln(stderr, "-list-sources and -delete-source read no input; drop -dir and the file argument")
			return 2
		}
		if *listSources && *deleteSource != "" {
			fmt.Fprintln(stderr, "use -list-sources or -delete-source, not both")
			return 2
		}
		if *indexPath == "" {
			fmt.Fprintln(stderr, "-index is required")
			return 2
		}
		st, closeIndex, err := openIndex(*indexPath)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer closeIndex()
		if *listSources {
			return runListSources(st, stdout, stderr)
		}
		return runDeleteSource(st, *deleteSource, stdout, stderr)
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

	// -rules names a file holding a JSON rule hierarchy. Read it here so
	// buildchunk sees only the text, not where it came from.
	var rulesText string
	if *rulesPath != "" {
		b, err := os.ReadFile(*rulesPath)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		rulesText = string(b)
	}

	c, err := buildchunk.New(*chunkerName, *tokName, *lang, rulesText, *size, *overlap, emb)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	var tok tokenizer.Tokenizer
	if *contextN != 0 {
		tok, err = buildchunk.NewTokenizer(*tokName)
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

	// One batch call for every input, not one Chunk call per file: the
	// results come back indexed like the jobs, and the error names the
	// input that failed. Tabular jobs (CSV/TSV) are excluded — their
	// rows are already the chunks, and running a chunker over them
	// would cut rows apart.
	texts := make([]string, 0, len(jobs))
	positions := make([]int, len(jobs)) // job index -> batch index, -1 for tabular
	for i, job := range jobs {
		if job.raw {
			positions[i] = -1
			continue
		}
		positions[i] = len(texts)
		texts = append(texts, job.text)
	}
	batched, err := batch.Chunk(c, texts)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	var all []chunk.Chunk
	docs := make([]chunk.Document, 0, len(jobs))
	for i, job := range jobs {
		var chunks []chunk.Chunk
		if job.raw {
			chunks, err = readTabular(job.sep, strings.NewReader(job.text))
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
		} else {
			chunks = batched[positions[i]]
		}
		if *contextN != 0 {
			var err error
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
		all = append(all, chunks...)
		docs = append(docs, chunk.NewDocument(job.path, job.text, chunks))
	}

	// Embedding runs once over every chunk of every input, after overlap so
	// a vector sees the same text store.Index would embed, including any
	// Context. Doing it per job would send one batch per file, which for the
	// OpenAI embedder is one request per file.
	if *embedInJSON {
		embedded, err := store.Embed(emb, all)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		all = embedded
		// docs holds per-file chunks, so hand each one back the slice that
		// was embedded. Slicing a nil all (no inputs) yields an empty set.
		at := 0
		for i := range docs {
			n := len(docs[i].Chunks)
			docs[i].Chunks = all[at : at+n : at+n]
			at += n
		}
	}

	if *indexPath != "" {
		st, closeIndex, err := openIndex(*indexPath)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		err = indexChunks(st, emb, all, *embedInJSON, *sourceName)
		if cerr := closeIndex(); err == nil {
			err = cerr
		}
		if err != nil {
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

// indexChunks writes chunks to st, labeled with src as their origin
// (empty means no source). When the chunks already carry vectors from
// -embed they are reused, so no second batch is sent. The labeled
// helpers keep the one definition of what gets embedded; the unlabeled
// ones are src "" of the same call.
func indexChunks(st store.Store, emb embed.Embedder, chunks []chunk.Chunk, preEmbedded bool, src string) error {
	if preEmbedded {
		return store.IndexEmbeddedLabeled(st, chunks, src)
	}
	return store.IndexLabeled(st, emb, chunks, src)
}

// runListSources prints the sources an index holds as JSON.
func runListSources(st store.Store, stdout, stderr io.Writer) int {
	srcs, err := store.Sources(st)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(srcs); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

// runDeleteSource removes one source from the index and prints the
// number of records that went.
func runDeleteSource(st store.Store, src string, stdout, stderr io.Writer) int {
	n, err := store.DeleteSource(st, src)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	if err := enc.Encode(struct {
		Deleted int `json:"deleted"`
	}{n}); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
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
	// raw marks tabular input (CSV/TSV): the text is already a set of
	// rows, so the chunker must not run over it. The chunks are parsed
	// in readTabular with the delimiter named here.
	raw bool
	sep rune
	// extract names the container format to pull plain text from
	// before chunking: "html" or "epub". Empty reads the bytes as the
	// text itself. Extraction happens in collectJobs, so by the time
	// jobs reach the batch the text is plain and every later stage is
	// format-blind.
	extract string
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
			job, err := makeJob(rel, b)
			if err != nil {
				return nil, err
			}
			jobs = append(jobs, job)
		}
		return jobs, nil
	}
	text, err := readInput(files, stdin)
	if err != nil {
		return nil, err
	}
	// A file argument can name a tabular or container format; stdin has
	// no name, so it always reads as plain text.
	if len(files) == 1 {
		job, err := makeJob(files[0], []byte(text))
		if err != nil {
			return nil, err
		}
		return []inputJob{job}, nil
	}
	return []inputJob{{path: "", text: text}}, nil
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
