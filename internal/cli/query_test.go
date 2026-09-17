package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/bluesky585/nibble/pkg/store"
)

// buildIndex writes chunks for original into a JSONL index at path.
func buildIndex(t *testing.T, path, original string, size int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"-chunker", "sentence", "-size", strconv.Itoa(size), "-index", path},
		strings.NewReader(original), &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("build index: exit %d stderr=%s", code, stderr.String())
	}
}

func TestRunQuery(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "idx.jsonl")
	buildIndex(t, path, "Cats sleep on mats. Quantum chromodynamics is hard.", 5)

	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"-query", "cats", "-index", path, "-k", "1"},
		strings.NewReader(""), &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var hits []store.Hit
	if err := json.Unmarshal(stdout.Bytes(), &hits); err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("len=%d want 1", len(hits))
	}
	if !strings.Contains(hits[0].Record.Chunk.Text, "Cats") {
		t.Fatalf("top hit=%q", hits[0].Record.Chunk.Text)
	}
}

// -query reads no input, so it must not block on stdin.
func TestRunQueryIgnoresStdin(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "idx.jsonl")
	buildIndex(t, path, "alpha beta. gamma delta.", 5)

	var stdout, stderr bytes.Buffer
	// A reader that would panic if read.
	code := Run(
		[]string{"-query", "alpha", "-index", path},
		panicReader{}, &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
}

func TestRunQueryMissingIndex(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-query", "cats"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d want 2 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "-index") {
		t.Fatalf("stderr=%s", stderr.String())
	}
}

func TestRunQueryEmptyIndex(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "empty.jsonl")
	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"-query", "cats", "-index", path},
		strings.NewReader(""), &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	// An empty index yields an empty array, not null.
	if strings.TrimSpace(stdout.String()) != "[]" {
		t.Fatalf("stdout=%s", stdout.String())
	}
}

func TestRunQueryBadK(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "idx.jsonl")
	buildIndex(t, path, "alpha beta.", 5)

	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"-query", "alpha", "-index", path, "-k", "0"},
		strings.NewReader(""), &stdout, &stderr,
	)
	if code != 2 {
		t.Fatalf("exit %d want 2 stderr=%s", code, stderr.String())
	}
}

// -query reads no input, so pairing it with -dir is a mistake rather
// than a silent ignore.
func TestRunQueryWithDir(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "idx.jsonl")
	buildIndex(t, path, "alpha beta.", 64)

	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"-query", "alpha", "-index", path, "-dir", t.TempDir()},
		strings.NewReader(""), &stdout, &stderr,
	)
	if code != 2 {
		t.Fatalf("exit %d want 2 stderr=%s", code, stderr.String())
	}
}

// -k without -query is a misuse, not a silent no-op.
func TestRunKWithoutQuery(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-k", "3"}, strings.NewReader("hi"), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit %d want 2 stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "-query") {
		t.Fatalf("stderr=%s", stderr.String())
	}
}

// A larger k than the index holds must not fail or pad.
func TestRunQueryKOverIndexSize(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "idx.jsonl")
	buildIndex(t, path, "alpha beta.", 64)

	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"-query", "alpha", "-index", path, "-k", "99"},
		strings.NewReader(""), &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	var hits []store.Hit
	if err := json.Unmarshal(stdout.Bytes(), &hits); err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("len=%d want 1", len(hits))
	}
}

// A round trip: index with -embed -index, then query the same file.
// Sentences are kept whole (size 64) so each hit is one sentence.
func TestRunIndexThenQuery(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "idx.jsonl")
	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"-chunker", "sentence", "-size", "64", "-embed", "-index", path},
		strings.NewReader("Cats sleep on mats. Quantum chromodynamics is hard."), &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(
		[]string{"-query", "chromodynamics", "-index", path, "-k", "1"},
		strings.NewReader(""), &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	var hits []store.Hit
	if err := json.Unmarshal(stdout.Bytes(), &hits); err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || !strings.Contains(hits[0].Record.Chunk.Text, "Quantum") {
		t.Fatalf("want the quantum sentence, got %+v", hits)
	}
	if hits[0].Score <= 0 {
		t.Fatalf("hit should score above zero: %+v", hits[0])
	}
}

// The index must still be readable as chunks, not only as hits.
func TestRunQueryResultCarriesChunkOffsets(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "idx.jsonl")
	original := "Cats sleep. Quantum fields."
	buildIndex(t, path, original, 5)

	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"-query", "cats", "-index", path, "-k", "1"},
		strings.NewReader(""), &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	var hits []store.Hit
	if err := json.Unmarshal(stdout.Bytes(), &hits); err != nil {
		t.Fatal(err)
	}
	got := hits[0].Record.Chunk
	runes := []rune(original)
	if string(runes[got.Start:got.End]) != got.Text {
		t.Fatalf("hit offsets do not match its text: %+v", got)
	}
}

// panicReader fails the test if anything reads it, proving -query never
// touches stdin.
type panicReader struct{}

func (panicReader) Read([]byte) (int, error) {
	panic("stdin was read")
}

// bm25 scoring needs no embedder at all: a query on an index built
// without vectors still ranks by term overlap. Here the quantum
// sentence must win with -scoring bm25 even though the hashing
// embedder is the only one available.
func TestRunQueryBM25(t *testing.T) {
	t.Parallel()

	// Size 64 keeps each sentence whole: BM25 scores terms, and a
	// chunk that holds half a word holds no term at all.
	path := filepath.Join(t.TempDir(), "idx.jsonl")
	buildIndex(t, path, "Cats sleep on mats. Quantum chromodynamics is hard.", 64)

	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"-query", "chromodynamics", "-index", path, "-k", "1", "-scoring", "bm25"},
		strings.NewReader(""), &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	var hits []store.Hit
	if err := json.Unmarshal(stdout.Bytes(), &hits); err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || !strings.Contains(hits[0].Record.Chunk.Text, "Quantum") {
		t.Fatalf("want the quantum sentence, got %+v", hits)
	}
	if hits[0].Score <= 0 {
		t.Fatalf("bm25 hit should score above zero: %+v", hits[0])
	}
}

// Hybrid blends both paths. With the default weight it agrees with the
// dense ranking on an obvious query, and an out-of-range weight is
// rejected.
func TestRunQueryHybrid(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "idx.jsonl")
	buildIndex(t, path, "Cats sleep on mats. Quantum chromodynamics is hard.", 5)

	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"-query", "cats sleep", "-index", path, "-k", "1", "-scoring", "hybrid"},
		strings.NewReader(""), &stdout, &stderr,
	)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}
	var hits []store.Hit
	if err := json.Unmarshal(stdout.Bytes(), &hits); err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || !strings.Contains(hits[0].Record.Chunk.Text, "Cats") {
		t.Fatalf("want the cats sentence, got %+v", hits)
	}

	stderr.Reset()
	code = Run(
		[]string{"-query", "cats", "-index", path, "-scoring", "hybrid", "-hybrid-weight", "2"},
		strings.NewReader(""), &stdout, &stderr,
	)
	if code != 2 || !strings.Contains(stderr.String(), "weight") {
		t.Fatalf("weight 2 accepted: exit=%d stderr=%s", code, stderr.String())
	}
}

// An unknown scoring name is a usage error, not a silent dense.
func TestRunQueryBadScoring(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "idx.jsonl")
	buildIndex(t, path, "alpha beta.", 5)

	var stdout, stderr bytes.Buffer
	code := Run(
		[]string{"-query", "alpha", "-index", path, "-scoring", "splade"},
		strings.NewReader(""), &stdout, &stderr,
	)
	if code != 2 || !strings.Contains(stderr.String(), "scoring") {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
}
