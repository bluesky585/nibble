package visualize

import (
	"strings"
	"testing"

	"github.com/bluesky585/nibble/pkg/chunk"
)

func mkChunk(t *testing.T, text string, start, end, tokens int) chunk.Chunk {
	t.Helper()
	c, err := chunk.New(text, start, end, tokens)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// segmentsText concatenates segment texts, which must always equal the
// original regardless of what the chunks look like.
func segmentsText(d Doc) string {
	var b strings.Builder
	for _, s := range d.Segments {
		b.WriteString(s.Text)
	}
	return b.String()
}

func TestNewDocTilesOriginal(t *testing.T) {
	t.Parallel()

	original := "Hello. World."
	chunks := []chunk.Chunk{
		mkChunk(t, "Hello. ", 0, 7, 7),
		mkChunk(t, "World.", 7, 13, 6),
	}
	d := NewDoc("", original, chunks)
	if got := segmentsText(d); got != original {
		t.Fatalf("segments=%q want %q", got, original)
	}
	if len(d.Segments) != 2 {
		t.Fatalf("len=%d want 2: %+v", len(d.Segments), d.Segments)
	}
	for i, s := range d.Segments {
		if !s.Chunk || s.Index != i {
			t.Fatalf("segment %d: %+v", i, s)
		}
	}
	if d.Segments[0].Slot != 0 || d.Segments[1].Slot != 1 {
		t.Fatalf("slots %d %d", d.Segments[0].Slot, d.Segments[1].Slot)
	}
}

// Chunks that skip text leave a gap segment; the page still shows the
// whole source.
func TestNewDocGap(t *testing.T) {
	t.Parallel()

	original := "aaa bbb ccc"
	chunks := []chunk.Chunk{
		mkChunk(t, "aaa ", 0, 4, 4),
		mkChunk(t, "ccc", 8, 11, 3),
	}
	d := NewDoc("", original, chunks)
	if got := segmentsText(d); got != original {
		t.Fatalf("segments=%q want %q", got, original)
	}
	var gaps []Segment
	for _, s := range d.Segments {
		if !s.Chunk {
			gaps = append(gaps, s)
		}
	}
	if len(gaps) != 1 || gaps[0].Text != "bbb " {
		t.Fatalf("gaps=%+v", gaps)
	}
}

// Overlapping chunks are shown once: the page keeps reading as the
// source, and the repeated part is flagged.
func TestNewDocOverlap(t *testing.T) {
	t.Parallel()

	original := "abcdef"
	chunks := []chunk.Chunk{
		mkChunk(t, "abcd", 0, 4, 4),
		mkChunk(t, "cdef", 2, 6, 4),
	}
	d := NewDoc("", original, chunks)
	if got := segmentsText(d); got != original {
		t.Fatalf("segments=%q want %q", got, original)
	}
	// The second chunk only contributes its uncovered tail.
	tail := d.Segments[len(d.Segments)-1]
	if tail.Text != "ef" || tail.Index != 1 {
		t.Fatalf("tail=%+v", tail)
	}
	if !tail.Overlap {
		t.Fatalf("tail should be marked overlapping: %+v", tail)
	}
	if d.Segments[0].Overlap {
		t.Fatalf("first chunk cannot overlap anything: %+v", d.Segments[0])
	}
	// Both chunks are still reported in the legend.
	if len(d.Legend) != 2 || d.Legend[0].Covered || d.Legend[1].Covered {
		t.Fatalf("legend=%+v", d.Legend)
	}
}

// A chunk fully swallowed by earlier ones contributes no text, and the
// legend says so.
func TestNewDocCovered(t *testing.T) {
	t.Parallel()

	original := "abcdef"
	chunks := []chunk.Chunk{
		mkChunk(t, "abcdef", 0, 6, 6),
		mkChunk(t, "cd", 2, 4, 2),
	}
	d := NewDoc("", original, chunks)
	if got := segmentsText(d); got != original {
		t.Fatalf("segments=%q want %q", got, original)
	}
	if !d.Legend[1].Covered {
		t.Fatalf("second chunk should be covered: %+v", d.Legend)
	}
	for _, s := range d.Segments {
		if s.Index == 1 && s.Chunk {
			t.Fatalf("covered chunk must not render text: %+v", s)
		}
	}
}

// A chunk whose offsets disagree with the source is flagged, not
// trusted. The page must not be built from bad offsets.
func TestNewDocMismatch(t *testing.T) {
	t.Parallel()

	original := "abcdef"
	bad := chunk.Chunk{Text: "zzz", Start: 1, End: 4, TokenCount: 3}
	d := NewDoc("", original, []chunk.Chunk{bad})
	if got := segmentsText(d); got != original {
		t.Fatalf("segments=%q want %q", got, original)
	}
	if !d.Legend[0].Mismatch {
		t.Fatalf("legend should flag the mismatch: %+v", d.Legend)
	}
	var found bool
	for _, s := range d.Segments {
		if s.Chunk && s.Mismatch {
			found = true
		}
	}
	if !found {
		t.Fatalf("segment should be flagged: %+v", d.Segments)
	}
}

func TestNewDocOutOfRange(t *testing.T) {
	t.Parallel()

	original := "abc"
	bad := chunk.Chunk{Text: "abcd", Start: 0, End: 99, TokenCount: 4}
	d := NewDoc("", original, []chunk.Chunk{bad})
	if got := segmentsText(d); got != original {
		t.Fatalf("segments=%q want %q", got, original)
	}
	if !d.Legend[0].Mismatch {
		t.Fatalf("legend=%+v", d.Legend)
	}
}

func TestNewDocEmpty(t *testing.T) {
	t.Parallel()

	d := NewDoc("", "", nil)
	if len(d.Segments) != 0 || len(d.Legend) != 0 {
		t.Fatalf("doc=%+v", d)
	}
	// No chunks over non-empty text is all gap.
	d = NewDoc("", "abc", nil)
	if len(d.Segments) != 1 || d.Segments[0].Chunk || d.Segments[0].Text != "abc" {
		t.Fatalf("segments=%+v", d.Segments)
	}
}

func TestNewDocUnicode(t *testing.T) {
	t.Parallel()

	original := "你好。世界！"
	chunks := []chunk.Chunk{
		mkChunk(t, "你好。", 0, 3, 3),
		mkChunk(t, "世界！", 3, 6, 3),
	}
	d := NewDoc("", original, chunks)
	if got := segmentsText(d); got != original {
		t.Fatalf("segments=%q want %q", got, original)
	}
	if d.RuneSize != 6 {
		t.Fatalf("runes=%d want 6", d.RuneSize)
	}
}

func TestNewDocCyclesSlots(t *testing.T) {
	t.Parallel()

	original := strings.Repeat("a", slots+3)
	var chunks []chunk.Chunk
	for i := 0; i < slots+3; i++ {
		chunks = append(chunks, mkChunk(t, "a", i, i+1, 1))
	}
	d := NewDoc("", original, chunks)
	if d.Segments[0].Slot != 0 {
		t.Fatalf("slot0=%d", d.Segments[0].Slot)
	}
	if d.Segments[slots].Slot != 0 {
		t.Fatalf("slot should wrap at %d, got %d", slots, d.Segments[slots].Slot)
	}
}

func TestHTML(t *testing.T) {
	t.Parallel()

	original := "<b>bold</b> & text"
	chunks := []chunk.Chunk{
		mkChunk(t, "<b>bold</b>", 0, 11, 11),
		mkChunk(t, " & text", 11, 18, 7),
	}
	out, err := HTML([]Doc{NewDoc("doc.md", original, chunks)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "<!DOCTYPE html>") {
		t.Fatalf("out=%q", out[:60])
	}
	// Source text is escaped, never injected as markup.
	if strings.Contains(out, "<b>bold</b>") {
		t.Fatal("source must be HTML-escaped")
	}
	if !strings.Contains(out, "&lt;b&gt;bold&lt;/b&gt;") {
		t.Fatal("escaped source not found")
	}
	if !strings.Contains(out, "doc.md") {
		t.Fatal("document name missing")
	}
	if !strings.Contains(out, "18 runes") {
		t.Fatal("rune count missing")
	}
}

// Every class the renderer can emit must exist in the stylesheet.
func TestHTMLPaletteDefined(t *testing.T) {
	t.Parallel()

	original := strings.Repeat("a", slots)
	var chunks []chunk.Chunk
	for i := 0; i < slots; i++ {
		chunks = append(chunks, mkChunk(t, "a", i, i+1, 1))
	}
	out, err := HTML([]Doc{NewDoc("", original, chunks)})
	if err != nil {
		t.Fatal(err)
	}
	for _, class := range paletteSlots() {
		if !strings.Contains(out, "."+class+" ") {
			t.Fatalf("stylesheet is missing .%s", class)
		}
	}
}

func TestHTMLEmpty(t *testing.T) {
	t.Parallel()

	out, err := HTML(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "<!DOCTYPE html>") {
		t.Fatalf("out=%q", out[:60])
	}
}

func TestHTMLLegendFlags(t *testing.T) {
	t.Parallel()

	original := "abcdef"
	chunks := []chunk.Chunk{
		mkChunk(t, "abc", 0, 3, 3),
		mkChunk(t, "zzz", 3, 6, 3),
	}
	chunks[1].Context = "header"
	out, err := HTML([]Doc{NewDoc("", original, chunks)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "mismatch") {
		t.Fatal("mismatch flag missing")
	}
	if !strings.Contains(out, "context") {
		t.Fatal("context flag missing")
	}
}
