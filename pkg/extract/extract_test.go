package extract

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Block-level tags become paragraph breaks; inline tags disappear; the
// visible text survives in reading order.
func TestHTMLBlockAndInline(t *testing.T) {
	t.Parallel()

	html := `<html><head><style>b { color: red }</style></head>` +
		`<body><h1>Title</h1><p>First <b>bold</b> para.</p>` +
		`<p>Second<br/>para line.</p></body></html>`
	got, err := HTML(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	// br separates a line within a paragraph, so it becomes a space;
	// only a block-level tag ends the paragraph itself.
	want := "Title\n\nFirst bold para.\n\nSecond para line."
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// script and style content is code, not text: it never reaches the
// output even when the document carries nothing else.
func TestHTMLSkipsScriptStyle(t *testing.T) {
	t.Parallel()

	html := `<body><script>var x = 1 < 2;</script>` +
		`<style>p { margin: 0 }</style>` +
		`<p>Visible</p></body>`
	got, err := HTML(strings.NewReader(html))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "var") || strings.Contains(got, "margin") {
		t.Fatalf("script/style leaked: %q", got)
	}
	if got != "Visible" {
		t.Fatalf("got %q", got)
	}
}

// Entities decode to their characters.
func TestHTMLEntities(t *testing.T) {
	t.Parallel()

	got, err := HTML(strings.NewReader("<p>Tom &amp; Jerry &lt;3</p>"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "Tom & Jerry <3" {
		t.Fatalf("got %q", got)
	}
}

// Malformed markup still extracts what it can: the tokenizer repairs
// an unclosed tag, and text is not lost to a parse error.
func TestHTMLMalformed(t *testing.T) {
	t.Parallel()

	got, err := HTML(strings.NewReader("<p>Unclosed <b>bold"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Unclosed") || !strings.Contains(got, "bold") {
		t.Fatalf("text lost in malformed HTML: %q", got)
	}
}

// Empty input yields empty text, not an error.
func TestHTMLEmpty(t *testing.T) {
	t.Parallel()

	got, err := HTML(strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("got %q", got)
	}
}

// buildEPUB zips the minimum container a reader expects: a rootfile
// declaration pointing at an OPF, which lists XHTML documents in order.
func buildEPUB(t *testing.T, dir string, docs map[string]string) string {
	t.Helper()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	opf := `<?xml version="1.0"?><package xmlns="http://www.idpf.org/2007/opf" unique-identifier="id">` +
		`<manifest><item id="c1" href="ch1.xhtml" media-type="application/xhtml+xml"/>` +
		`<item id="c2" href="ch2.xhtml" media-type="application/xhtml+xml"/></manifest>` +
		`<spine><itemref idref="c1"/><itemref idref="c2"/></spine></package>`
	names := []string{"META-INF/container.xml", "content.opf", "ch1.xhtml", "ch2.xhtml"}
	contents := map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">` +
			`<rootfiles><rootfile full-path="content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`,
		"content.opf": opf,
		"ch1.xhtml":   docs["ch1.xhtml"],
		"ch2.xhtml":   docs["ch2.xhtml"],
	}
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(contents[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "book.epub")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// An EPUB extracts its spine documents in order, joined by paragraph
// breaks, so the chapters read as one document.
func TestEPUBSpineOrder(t *testing.T) {
	t.Parallel()

	path := buildEPUB(t, t.TempDir(), map[string]string{
		"ch1.xhtml": "<html><body><p>One one.</p><p>Two two.</p></body></html>",
		"ch2.xhtml": "<html><body><p>Three three.</p></body></html>",
	})

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := EPUB(f)
	if err != nil {
		t.Fatal(err)
	}
	want := "One one.\n\nTwo two.\n\nThree three."
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// An archive that is not an EPUB — no container.xml — is an error
// rather than silent empty output.
func TestEPUBMissingContainer(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("readme.txt")
	w.Write([]byte("not a book"))
	zw.Close()

	got, err := EPUB(bytes.NewReader(buf.Bytes()))
	if err == nil {
		t.Fatalf("expected an error, got %q", got)
	}
}

// A file that is not a zip at all is an error naming the cause.
func TestEPUBNotAZip(t *testing.T) {
	t.Parallel()

	if _, err := EPUB(strings.NewReader("plain text")); err == nil {
		t.Fatal("expected an error")
	}
}
