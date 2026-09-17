package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bluesky585/nibble/pkg/chunk"
)

// A CSV file read through -dir yields one chunk per data row, with the
// column names carried in Context so retrieval keeps the field names,
// the way the table chunker carries a GFM header.
func TestRunDirCSV(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "rows.csv")
	content := "name,color\napple,red\nbanana,yellow\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-dir", dir, "-ext", ".csv", "-size", "512"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var docs []chunk.Document
	if err := json.Unmarshal(stdout.Bytes(), &docs); err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("docs=%d want 1", len(docs))
	}
	got := docs[0].Chunks
	if len(got) != 2 {
		t.Fatalf("chunks=%d want 2 (one per data row)", len(got))
	}
	if got[0].Text != "apple,red" {
		t.Fatalf("chunk0=%q want \"apple,red\"", got[0].Text)
	}
	if got[0].Context != "name,color" {
		t.Fatalf("chunk0 context=%q want \"name,color\"", got[0].Context)
	}
	if got[1].Text != "banana,yellow" || got[1].Context != "name,color" {
		t.Fatalf("chunk1=%q context=%q", got[1].Text, got[1].Context)
	}
	// The header row itself is metadata, not content: it must not be
	// emitted as a chunk of its own.
	for i, ch := range got {
		if ch.Text == "name,color" {
			t.Fatalf("chunk %d is the header row", i)
		}
	}
}

// Quoted fields, embedded commas, and embedded newlines are CSV's whole
// point. A row whose field holds a newline still becomes one chunk, and
// the values come through decoded rather than quoted.
func TestRunDirCSVQuotedFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "rows.csv")
	content := "name,note\n\"Widget, large\",\"two\nlines\"\nplain,simple\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-dir", dir, "-ext", ".csv", "-size", "512"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var docs []chunk.Document
	if err := json.Unmarshal(stdout.Bytes(), &docs); err != nil {
		t.Fatal(err)
	}
	got := docs[0].Chunks
	if len(got) != 2 {
		t.Fatalf("chunks=%d want 2", len(got))
	}
	if got[0].Text != "Widget, large,two\nlines" {
		t.Fatalf("chunk0=%q", got[0].Text)
	}
	if got[1].Text != "plain,simple" {
		t.Fatalf("chunk1=%q", got[1].Text)
	}
}

// TSV follows the same rules with a tab separator.
func TestRunDirTSV(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "rows.tsv")
	content := "name\tcolor\napple\tred\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-dir", dir, "-ext", ".tsv"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var docs []chunk.Document
	if err := json.Unmarshal(stdout.Bytes(), &docs); err != nil {
		t.Fatal(err)
	}
	got := docs[0].Chunks
	if len(got) != 1 || got[0].Text != "apple\tred" {
		t.Fatalf("chunks=%+v", got)
	}
	if got[0].Context != "name\tcolor" {
		t.Fatalf("context=%q", got[0].Context)
	}
}

// A CSV file named as a plain file argument is parsed the same way; the
// path does not have to go through -dir.
func TestRunCSVFileArg(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "rows.csv")
	content := "name,color\napple,red\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-chunker", "token", "-size", "512", path}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var chunks []chunk.Chunk
	if err := json.Unmarshal(stdout.Bytes(), &chunks); err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 1 || chunks[0].Text != "apple,red" {
		t.Fatalf("chunks=%+v", chunks)
	}
	if chunks[0].Context != "name,color" {
		t.Fatalf("context=%q", chunks[0].Context)
	}
}

// A file with a CSV extension that does not parse as CSV is an error,
// not silently-empty output.
func TestRunCSVBadFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "broken.csv")
	// A quote opens and never closes: encoding/csv reports parse.Error.
	if err := os.WriteFile(path, []byte("name,note\n\"unterminated"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-dir", dir, "-ext", ".csv"}, strings.NewReader(""), &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected nonzero exit, stdout=%s", stdout.String())
	}
}

// A row wider than the budget still becomes one chunk: the row is the
// unit of meaning in tabular data, and cutting one in half destroys the
// column-to-value pairing. Oversize rows are the table chunker's trade
// too.
func TestRunCSVRowOverBudget(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "rows.csv")
	content := "name,note\nword," + strings.Repeat("x", 40) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-dir", dir, "-ext", ".csv", "-size", "8"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var docs []chunk.Document
	if err := json.Unmarshal(stdout.Bytes(), &docs); err != nil {
		t.Fatal(err)
	}
	got := docs[0].Chunks
	if len(got) != 1 {
		t.Fatalf("chunks=%d want 1 (a row is never split)", len(got))
	}
	if got[0].TokenCount != 45 {
		t.Fatalf("tokens=%d want 45 (row kept whole)", got[0].TokenCount)
	}
}
