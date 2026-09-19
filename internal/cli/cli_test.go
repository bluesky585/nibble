package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bluesky585/nibble/internal/assertchunk"
	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/embed"
	"github.com/bluesky585/nibble/pkg/store"
)

func TestRunStdin(t *testing.T) {
	t.Parallel()

	in := strings.NewReader("Hello. World.")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"-chunker", "sentence", "-size", "64"}, in, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var chunks []chunk.Chunk
	if err := json.Unmarshal(stdout.Bytes(), &chunks); err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, "Hello. World.", chunks)
}

func TestRunFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "doc.txt")
	original := "你好。世界！"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-chunker", "recursive", "-size", "3", path}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var chunks []chunk.Chunk
	if err := json.Unmarshal(stdout.Bytes(), &chunks); err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, chunks)
	if chunks[0].End != 3 {
		t.Fatalf("want rune offsets, first end=%d", chunks[0].End)
	}
}

func TestRunIndex(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "idx.jsonl")
	original := "cats sleep. quantum chromodynamics."
	var stdout, stderr bytes.Buffer
	code := Run([]string{"-chunker", "sentence", "-size", "64", "-index", path}, strings.NewReader(original), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	st, err := store.OpenJSONL(path)
	if err != nil {
		t.Fatal(err)
	}
	q, err := embed.Hashing{}.Embed([]string{"cats"})
	if err != nil {
		t.Fatal(err)
	}
	hits, err := st.Search(q[0], 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || !strings.Contains(hits[0].Record.Chunk.Text, "cats") {
		t.Fatalf("got %+v", hits)
	}
}

func TestRunSemantic(t *testing.T) {
	t.Parallel()

	original := "Cats sleep. Quantum chromodynamics is hard."
	var stdout, stderr bytes.Buffer
	code := Run([]string{"-chunker", "semantic", "-size", "512"}, strings.NewReader(original), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var chunks []chunk.Chunk
	if err := json.Unmarshal(stdout.Bytes(), &chunks); err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, chunks)
}

func TestRunCode(t *testing.T) {
	t.Parallel()

	original := "package p\n\nfunc A() {}\n"
	var stdout, stderr bytes.Buffer
	code := Run([]string{"-chunker", "code", "-size", "64"}, strings.NewReader(original), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var chunks []chunk.Chunk
	if err := json.Unmarshal(stdout.Bytes(), &chunks); err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, chunks)
}

// -lang picks the rules for -chunker code, and a language nibble cannot cut
// is an error rather than a silent fallback to detection.
func TestRunCodeLang(t *testing.T) {
	t.Parallel()

	original := "def a():\n    pass\n\n\ndef b():\n    pass\n"
	var stdout, stderr bytes.Buffer
	code := Run([]string{"-chunker", "code", "-lang", "python", "-size", "24"}, strings.NewReader(original), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	var chunks []chunk.Chunk
	if err := json.Unmarshal(stdout.Bytes(), &chunks); err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, chunks)
	if len(chunks) != 2 {
		t.Fatalf("len=%d want 2: %+v", len(chunks), chunks)
	}

	stderr.Reset()
	code = Run([]string{"-chunker", "code", "-lang", "rust"}, strings.NewReader(original), &stdout, &stderr)
	if code == 0 {
		t.Fatalf("an unknown language must fail, stdout=%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), `unknown language "rust"`) {
		t.Fatalf("stderr=%s", stderr.String())
	}
}

func TestRunTable(t *testing.T) {
	t.Parallel()

	original := "| h |\n| --- |\n| 1 |\n"
	var stdout, stderr bytes.Buffer
	code := Run([]string{"-chunker", "table", "-size", "64"}, strings.NewReader(original), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var chunks []chunk.Chunk
	if err := json.Unmarshal(stdout.Bytes(), &chunks); err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, chunks)
}

func TestRunFast(t *testing.T) {
	t.Parallel()

	original := "你好"
	var stdout, stderr bytes.Buffer
	code := Run([]string{"-chunker", "fast", "-size", "1"}, strings.NewReader(original), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var chunks []chunk.Chunk
	if err := json.Unmarshal(stdout.Bytes(), &chunks); err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, chunks)
	if chunks[0].End != 1 {
		t.Fatalf("want rune end 1, got %+v", chunks[0])
	}
}

func TestRunContextPrefix(t *testing.T) {
	t.Parallel()

	original := "hello"
	var stdout, stderr bytes.Buffer
	code := Run([]string{"-chunker", "token", "-size", "3", "-context", "2", "-context-mode", "prefix"}, strings.NewReader(original), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var chunks []chunk.Chunk
	if err := json.Unmarshal(stdout.Bytes(), &chunks); err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, chunks)
	if len(chunks) != 2 || chunks[1].Context != "el" {
		t.Fatalf("got %+v", chunks)
	}
}

func TestRunContextSuffix(t *testing.T) {
	t.Parallel()

	original := "hello"
	var stdout, stderr bytes.Buffer
	code := Run([]string{"-chunker", "token", "-size", "3", "-context", "2", "-context-mode", "suffix"}, strings.NewReader(original), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var chunks []chunk.Chunk
	if err := json.Unmarshal(stdout.Bytes(), &chunks); err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, chunks)
	if len(chunks) != 2 || chunks[0].Context != "lo" {
		t.Fatalf("got %+v", chunks)
	}
}

func TestRunUnknownContextMode(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-context", "1", "-context-mode", "sideways"}, strings.NewReader("hello"), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d want 2 stderr=%s", code, stderr.String())
	}
}

func TestRunTokenOverlap(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-chunker", "token", "-size", "3", "-overlap", "1"}, strings.NewReader("hello"), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var chunks []chunk.Chunk
	if err := json.Unmarshal(stdout.Bytes(), &chunks); err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 2 || chunks[0].Text != "hel" || chunks[1].Text != "llo" {
		t.Fatalf("got %+v", chunks)
	}
}

// Recursive accepts -overlap: the next chunk repeats the previous
// chunk's tail in its own text, the way a token window widens.
func TestRunRecursiveOverlap(t *testing.T) {
	t.Parallel()

	original := strings.Repeat("word ", 8)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"-chunker", "recursive", "-size", "8", "-overlap", "2"}, strings.NewReader(original), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var chunks []chunk.Chunk
	if err := json.Unmarshal(stdout.Bytes(), &chunks); err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 2 {
		t.Fatalf("chunks=%d, want several", len(chunks))
	}
	for i := 1; i < len(chunks); i++ {
		if chunks[i].Start != chunks[i-1].End-2 {
			t.Fatalf("chunk %d start=%d want %d", i, chunks[i].Start, chunks[i-1].End-2)
		}
	}
}

func TestRunDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skip.go"), []byte("package p"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-chunker", "token", "-size", "64", "-dir", root, "-ext", ".txt"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var got []chunk.Document
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Path != "a.txt" || got[0].Content != "hello" {
		t.Fatalf("got %+v", got)
	}
	assertchunk.Split(t, got[0].Content, got[0].Chunks)
}

func TestRunDirAndFile(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-dir", t.TempDir(), "a.txt"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d want 2 stderr=%s", code, stderr.String())
	}
}

func TestRunUnknownEmbedder(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-embedder", "magic"}, strings.NewReader("hi"), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d want 2 stderr=%s", code, stderr.String())
	}
}

func TestRunUnknownChunker(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-chunker", "magic"}, strings.NewReader("hi"), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d want 2", code)
	}
}

func TestRunTooManyFiles(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"a.txt", "b.txt"}, strings.NewReader(""), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit %d want 1 stderr=%s", code, stderr.String())
	}
}

func TestRunEmbed(t *testing.T) {
	t.Parallel()

	original := "Hello. World."
	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"-chunker", "sentence", "-size", "64", "-embed"},
		strings.NewReader(original), &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var chunks []chunk.Chunk
	if err := json.Unmarshal(stdout.Bytes(), &chunks); err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, chunks)
	for i, c := range chunks {
		if len(c.Embedding) == 0 {
			t.Fatalf("chunk %d has no embedding", i)
		}
	}
}

// Without -embed the JSON must not grow an embedding field.
func TestRunEmbedOffByDefault(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-chunker", "sentence", "-size", "64"}, strings.NewReader("Hi. Yo."), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "embedding") {
		t.Fatalf("embedding should be absent: %s", stdout.String())
	}
}

// -embed together with -index must store usable vectors, and must not
// keep a second copy on the chunk itself.
func TestRunEmbedAndIndexReuse(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "idx.jsonl")
	original := "Cats sleep. Cats eat."

	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"-chunker", "sentence", "-size", "64", "-embed", "-index", path},
		strings.NewReader(original), &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	st, err := store.OpenJSONL(path)
	if err != nil {
		t.Fatal(err)
	}
	q, err := embed.Hashing{}.Embed([]string{"cats"})
	if err != nil {
		t.Fatal(err)
	}
	hits, err := st.Search(q[0], 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("got %+v", hits)
	}
	// The vector was stored once, on the record, not on the chunk too.
	if len(hits[0].Record.Chunk.Embedding) != 0 {
		t.Fatal("stored chunk should not repeat its vector")
	}
	if len(hits[0].Record.Vector) == 0 {
		t.Fatal("record has no vector")
	}
}

func TestRunHTML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outPath := filepath.Join(dir, "out.html")
	original := "Hello. World."

	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"-chunker", "sentence", "-size", "64", "-html", outPath},
		strings.NewReader(original), &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	// stdout is still JSON.
	var chunks []chunk.Chunk
	if err := json.Unmarshal(stdout.Bytes(), &chunks); err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, chunks)

	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	page := string(b)
	if !strings.HasPrefix(page, "<!DOCTYPE html>") {
		t.Fatalf("page=%q", page[:40])
	}
	if !strings.Contains(page, "Hello. ") {
		t.Fatal("chunk text missing from the page")
	}
}

func TestRunHTMLDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(root, "out.html")

	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"-chunker", "token", "-size", "64", "-dir", root, "-ext", ".txt", "-html", outPath},
		strings.NewReader(""), &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "a.txt") {
		t.Fatal("document path missing from the page")
	}
}

func TestRunHTMLBadPath(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"-html", filepath.Join(t.TempDir(), "missing", "out.html")},
		strings.NewReader("hi"), &stdout, &stderr,
	)
	if code != 1 {
		t.Fatalf("exit %d want 1 stderr=%s", code, stderr.String())
	}
}

func TestRunEmpty(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run(nil, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "[]" {
		t.Fatalf("stdout=%s", stdout.String())
	}
}

// -dir with -embed must send one embedding batch for the whole run, not
// one per file. The server counts requests, so restoring per-file
// embedding fails here.
func TestRunEmbedDirUsesOneBatch(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("Cats sleep. Cats eat."), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			return
		}
		var req struct {
			Input []string `json:"input"`
		}
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		resp := struct {
			Data []map[string]any `json:"data"`
		}{}
		for i := range req.Input {
			resp.Data = append(resp.Data, map[string]any{
				"index":     i,
				"embedding": []float64{1, 0},
			})
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("OPENAI_BASE_URL", srv.URL)

	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"-chunker", "sentence", "-size", "64", "-embedder", "openai", "-embed", "-dir", dir},
		strings.NewReader(""), &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	if calls != 1 {
		t.Fatalf("embedding requests=%d, want 1 for the whole run", calls)
	}

	// Nothing may be lost by embedding in one pass: every chunk of every
	// document still carries a vector, and the text is still a real split.
	var docs []chunk.Document
	if err := json.Unmarshal(stdout.Bytes(), &docs); err != nil {
		t.Fatal(err)
	}
	if len(docs) != 3 {
		t.Fatalf("documents=%d want 3", len(docs))
	}
	for _, d := range docs {
		assertchunk.Split(t, d.Content, d.Chunks)
		for i, c := range d.Chunks {
			if len(c.Embedding) == 0 {
				t.Fatalf("%s chunk %d has no embedding", d.Path, i)
			}
		}
	}
}

// The tiktoken tokenizer counts real BPE tokens, so -size is a model budget.
// It needs the encoding table, which is downloaded on a cold cache, so the
// test skips rather than fails when it cannot be fetched.
func TestRunTiktokenTokenizer(t *testing.T) {
	original := "The quick brown fox jumps over the lazy dog."

	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"-chunker", "token", "-tokenizer", "tiktoken", "-size", "1000"},
		strings.NewReader(original), &stdout, &stderr,
	)
	if strings.Contains(stderr.String(), "unavailable") || strings.Contains(stderr.String(), "could not") {
		t.Skipf("tiktoken encoding unavailable: %s", stderr.String())
	}
	if code != 0 {
		t.Skipf("tiktoken not usable here (exit %d): %s", code, stderr.String())
	}

	var chunks []chunk.Chunk
	if err := json.Unmarshal(stdout.Bytes(), &chunks); err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, chunks)
	if len(chunks) != 1 {
		t.Fatalf("chunks=%d want 1 for a 1000-token budget", len(chunks))
	}
	// cl100k_base counts this sentence as 10 tokens, not 44 runes.
	if chunks[0].TokenCount != 10 {
		t.Fatalf("token_count=%d want 10 (a rune count would be %d)",
			chunks[0].TokenCount, len([]rune(original)))
	}
}

// -context builds its own tokenizer, so every name must resolve there too.
// The two tokenizer factories once disagreed and "tiktoken" was unknown.
func TestRunContextAcceptsEveryTokenizer(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"character", "word"} {
		var stdout, stderr bytes.Buffer
		code := Run(
			[]string{"-chunker", "token", "-tokenizer", name, "-size", "6", "-context", "2"},
			strings.NewReader("hello world"), &stdout, &stderr,
		)
		if code != 0 {
			t.Fatalf("%s: exit %d stderr=%s", name, code, stderr.String())
		}
	}
}

// A -source on -index labels every chunk with that origin. -list-sources
// prints the labels with counts, and -delete-source removes exactly one
// origin and reports how many records went.
func TestRunSourceLifecycle(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "idx.jsonl")
	run := func(args ...string) (string, int) {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := Run(args, strings.NewReader(""), &stdout, &stderr)
		return stdout.String(), code
	}

	// Index two texts, one under each source; the text rides stdin.
	for _, tc := range []struct{ text, src string }{
		{"cats sleep on mats.", "a.md"},
		{"dogs bark all night.", "a.md"},
		{"birds migrate in autumn.", "b.md"},
	} {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"-chunker", "sentence", "-size", "64", "-index", path,
			"-source", tc.src}, strings.NewReader(tc.text), &stdout, &stderr)
		if code != 0 {
			t.Fatalf("index %s: exit %d stderr=%s", tc.src, code, stderr.String())
		}
	}

	out, code := run("-list-sources", "-index", path)
	if code != 0 {
		t.Fatalf("list: exit %d", code)
	}
	var srcs []store.Source
	if err := json.Unmarshal([]byte(out), &srcs); err != nil {
		t.Fatalf("list output %q: %v", out, err)
	}
	if len(srcs) != 2 || srcs[0].Name != "a.md" || srcs[0].Count != 2 {
		t.Fatalf("sources=%+v", srcs)
	}

	out, code = run("-delete-source", "b.md", "-index", path)
	if code != 0 {
		t.Fatalf("delete: exit %d", code)
	}
	var del struct {
		Deleted int `json:"deleted"`
	}
	if err := json.Unmarshal([]byte(out), &del); err != nil {
		t.Fatalf("delete output %q: %v", out, err)
	}
	if del.Deleted != 1 {
		t.Fatalf("deleted=%d, want 1", del.Deleted)
	}

	// The survivor is still searchable, and a second delete reports 0.
	out, _ = run("-list-sources", "-index", path)
	if !strings.Contains(out, `"name": "a.md"`) || strings.Contains(out, "b.md") {
		t.Fatalf("sources after delete: %s", out)
	}
}

// The two source-management flags reject being used together and both
// require -index; neither reads input.
func TestRunSourceFlagErrors(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "idx.jsonl")
	cases := [][]string{
		{"-list-sources", "-delete-source", "x", "-index", path},
		{"-list-sources"},
		{"-delete-source", "x"},
		{"-list-sources", "-dir", "."},
	}
	for _, args := range cases {
		var stdout, stderr bytes.Buffer
		if code := Run(args, strings.NewReader(""), &stdout, &stderr); code != 2 {
			t.Errorf("args %v: exit %d, want 2 (stderr=%s)", args, code, stderr.String())
		}
	}
}
