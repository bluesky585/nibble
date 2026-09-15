package markdownchunker

import (
	"strings"
	"testing"

	"github.com/bluesky585/nibble/internal/assertchunk"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

func TestNewValidation(t *testing.T) {
	t.Parallel()

	if _, err := New(nil, 8); err == nil || !strings.Contains(err.Error(), "tokenizer is required") {
		t.Fatalf("err=%v", err)
	}
	if _, err := New(tokenizer.Character{}, 0); err == nil || !strings.Contains(err.Error(), "size must be > 0") {
		t.Fatalf("err=%v", err)
	}
}

func TestChunkOnlyProse(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 64)
	if err != nil {
		t.Fatal(err)
	}
	original := "plain paragraph\n\nsecond paragraph\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
}

func TestChunkOnlyTable(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 18)
	if err != nil {
		t.Fatal(err)
	}
	original := "| h |\n| --- |\n| 1 |\n| 2 |\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) < 2 {
		t.Fatalf("table should split, got %+v", got)
	}
	// The table chunker keeps the header on continuation chunks.
	if got[len(got)-1].Context == "" {
		t.Fatalf("continuation chunk needs header context, got %+v", got)
	}
}

func TestChunkOnlyCode(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 64)
	if err != nil {
		t.Fatal(err)
	}
	original := "```go\nfunc A() {}\n```\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) != 1 {
		t.Fatalf("len=%d want 1, got %+v", len(got), got)
	}
	// Fence lines are reattached, so text still covers the whole block.
	if got[0].Text != original {
		t.Fatalf("text=%q want %q", got[0].Text, original)
	}
	if !strings.Contains(got[0].Text, "```go") {
		t.Fatalf("opening fence missing: %q", got[0].Text)
	}
}

func TestChunkMixedKeepsReconstruct(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 24)
	if err != nil {
		t.Fatal(err)
	}
	original := strings.Join([]string{
		"# Title",
		"",
		"Some intro prose that is long enough to be split by the recursive chunker.",
		"",
		"| col |",
		"| --- |",
		"| 1 |",
		"| 2 |",
		"",
		"```go",
		"func A() {}",
		"```",
		"",
		"Outro prose.",
		"",
	}, "\n")

	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) < 3 {
		t.Fatalf("expected region splits, got %+v", got)
	}
}

func TestChunkEmpty(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 8)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Chunk("")
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, "", got)
}

// A pipe row inside a fence is code, not a table header.
func TestChunkTableInsideCodeIsCode(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 128)
	if err != nil {
		t.Fatal(err)
	}
	original := "```\n| h |\n| --- |\n| 1 |\n```\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) != 1 {
		t.Fatalf("len=%d want 1, got %+v", len(got), got)
	}
	if got[0].Context != "" {
		t.Fatalf("code chunk must not carry table header context: %q", got[0].Context)
	}
}

// A fence long enough to split must keep offsets contiguous across the
// interior chunks while the fences stay on the boundary chunks.
func TestChunkSplitCodeKeepsFences(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 20)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Repeat("x = 1\n", 12)
	original := "```go\n" + body + "```\n"

	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) < 2 {
		t.Fatalf("expected the block to split, got %+v", got)
	}
	if !strings.HasPrefix(got[0].Text, "```go\n") {
		t.Fatalf("first chunk must open with the fence: %q", got[0].Text)
	}
	if !strings.HasSuffix(got[len(got)-1].Text, "```\n") {
		t.Fatalf("last chunk must close with the fence: %q", got[len(got)-1].Text)
	}
	// Interior chunks carry no fence characters.
	for _, ch := range got[1 : len(got)-1] {
		if strings.Contains(ch.Text, "```") {
			t.Fatalf("interior chunk has a fence: %q", ch.Text)
		}
	}
}

func TestChunkUnicode(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 32)
	if err != nil {
		t.Fatal(err)
	}
	original := "前\n\n| 列 |\n| --- |\n| 值 |\n\n```\n你好\n```\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
}

func TestChunkUnclosedFenceIsProse(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 16)
	if err != nil {
		t.Fatal(err)
	}
	// No closing fence, so the block is not recognized and stays prose.
	original := "```go\nfunc A() {}\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
}

// A fence's info string picks the rules the block is cut by, and that choice
// beats detection.
//
// The body is Python, so the false boundary under test is not a subtle one:
// under ```go it does not parse as Go at all, and the block falls to token
// windows, which cut wherever the budget falls; under ```python it is cut on
// its two top-level defs. The assertion is on which chunk holds the second
// def, because that is what differs: detection would refuse to read a ```go
// block as Python, and reading a ```python block as Go is the bug this
// replaces, where the fence's own language was parsed and then thrown away.
func TestChunkFenceLanguageDecidesRules(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 24)
	if err != nil {
		t.Fatal(err)
	}
	body := "def a():\n    pass\n\n\ndef b():\n    pass\n"

	// A ```python block is cut on the def: the second chunk opens with it.
	// "py" is an alias, so it has to reach the same chunker.
	for _, lang := range []string{"python", "py"} {
		original := "```" + lang + "\n" + body + "```\n"
		got, err := c.Chunk(original)
		if err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		assertchunk.Split(t, original, got)
		if len(got) != 2 {
			t.Fatalf("%s: len=%d want 2: %+v", lang, len(got), got)
		}
		if !strings.HasPrefix(got[1].Text, "def b():") {
			t.Fatalf("%s: chunk 1 must open at the second def, got %q", lang, got[1].Text)
		}
	}

	// A ```go block is not: the same body falls to token windows, which do
	// not know where a def begins, so the second chunk opens mid-statement.
	original := "```go\n" + body + "```\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) < 2 {
		t.Fatalf("expected the block to split, got %+v", got)
	}
	if strings.HasPrefix(got[1].Text, "def b():") {
		t.Fatalf("chunk 1 opened at a Python def, so the fence's go was ignored: %q", got[1].Text)
	}
}
