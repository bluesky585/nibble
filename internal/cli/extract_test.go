package cli

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bluesky585/nibble/pkg/chunk"
)

// An HTML file named as a file argument is extracted to text before
// chunking: the chunks hold the visible text, not the markup.
func TestRunHTMLFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "page.html")
	content := "<html><body><h1>Title</h1><p>First para.</p><p>Second para.</p></body></html>"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	// Size 12 forces one chunk per paragraph; at the default 512 the
	// three short paragraphs pack into a single chunk.
	code := Run([]string{"-chunker", "recursive", "-size", "12", path}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var chunks []chunk.Chunk
	if err := json.Unmarshal(stdout.Bytes(), &chunks); err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 3 {
		t.Fatalf("chunks=%d want 3 (one per paragraph)", len(chunks))
	}
	// The recursive rules attach a paragraph break to the piece it ends,
	// so each chunk but the last carries its trailing \n\n.
	if chunks[0].Text != "Title\n\n" || chunks[1].Text != "First para.\n\n" || chunks[2].Text != "Second para." {
		t.Fatalf("got %+v", chunks)
	}
	// The extracted text is what was chunked, so the offsets are rune
	// offsets into that text, and reconstruct holds against it.
	var joined string
	for _, ch := range chunks {
		joined += ch.Text
	}
	if joined != "Title\n\nFirst para.\n\nSecond para." {
		t.Fatalf("joined=%q", joined)
	}
}

// An EPUB in a -dir sweep extracts and chunks like any other file.
func TestRunEPUBDir(t *testing.T) {
	t.Parallel()

	// Reuse the zip builder from the extract package tests by writing a
	// minimal book here.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"content.opf":            `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" unique-identifier="id"><manifest><item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="c1"/></spine></package>`,
		"ch1.xhtml":              "<html><body><p>Chapter text here.</p></body></html>",
	}
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(content))
	}
	zw.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "book.epub"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-dir", dir, "-ext", ".epub", "-size", "512"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%s", code, stderr.String())
	}

	var docs []chunk.Document
	if err := json.Unmarshal(stdout.Bytes(), &docs); err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 || len(docs[0].Chunks) != 1 {
		t.Fatalf("docs=%+v", docs)
	}
	if docs[0].Chunks[0].Text != "Chapter text here." {
		t.Fatalf("chunk=%q", docs[0].Chunks[0].Text)
	}
}

// An EPUB that is not a book is an error naming the file, even in a
// directory sweep where other files are fine.
func TestRunEPUBBadInDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.epub"), []byte("not a zip"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run([]string{"-dir", dir, "-ext", ".epub"}, strings.NewReader(""), &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected nonzero exit, stdout=%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "bad.epub") {
		t.Fatalf("stderr=%s", stderr.String())
	}
}
