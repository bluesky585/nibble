package cli

import (
	"bytes"
	"encoding/json"
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

func TestRunOverlapRejected(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-chunker", "recursive", "-overlap", "1"}, strings.NewReader("hi"), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d want 2 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "overlap is only supported by the token chunker") {
		t.Fatalf("stderr=%s", stderr.String())
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
