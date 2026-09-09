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
