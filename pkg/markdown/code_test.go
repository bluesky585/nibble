package markdown

import (
	"testing"
	"unicode/utf8"
)

func assertCodeSpan(t *testing.T, original string, c Code) {
	t.Helper()
	runes := []rune(original)
	if c.Start < 0 || c.End > len(runes) || c.Start > c.End {
		t.Fatalf("range [%d, %d) invalid for %d runes", c.Start, c.End, len(runes))
	}
	if string(runes[c.Start:c.End]) != c.Text {
		t.Fatalf("text %q != original[%d:%d]", c.Text, c.Start, c.End)
	}
}

func TestCodeBlocksNone(t *testing.T) {
	t.Parallel()

	if got := CodeBlocks(""); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
	if got := CodeBlocks("no fence\n"); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestCodeBlocksOne(t *testing.T) {
	t.Parallel()

	original := "intro\n```go\nfunc A() {}\n```\noutro\n"
	got := CodeBlocks(original)
	if len(got) != 1 {
		t.Fatalf("len=%d", len(got))
	}
	assertCodeSpan(t, original, got[0])
	if got[0].Language != "go" {
		t.Fatalf("lang=%q", got[0].Language)
	}
	if got[0].Text != "```go\nfunc A() {}\n```\n" {
		t.Fatalf("text=%q", got[0].Text)
	}
}

func TestCodeBlocksTwo(t *testing.T) {
	t.Parallel()

	original := "```\na\n```\n\n```py\nb\n```\n"
	got := CodeBlocks(original)
	if len(got) != 2 {
		t.Fatalf("len=%d got %+v", len(got), got)
	}
	assertCodeSpan(t, original, got[0])
	assertCodeSpan(t, original, got[1])
	if got[0].Language != "" || got[1].Language != "py" {
		t.Fatalf("langs %q %q", got[0].Language, got[1].Language)
	}
	if got[0].End > got[1].Start {
		t.Fatalf("overlap %+v %+v", got[0], got[1])
	}
}

func TestCodeBlocksUnclosed(t *testing.T) {
	t.Parallel()

	if got := CodeBlocks("```go\nfunc A() {}\n"); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestCodeBlocksUnicode(t *testing.T) {
	t.Parallel()

	original := "前\n```\n你好\n```\n"
	got := CodeBlocks(original)
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	assertCodeSpan(t, original, got[0])
	if utf8.RuneCountInString("前\n") != 2 {
		t.Fatal("precondition")
	}
	if got[0].Start != 2 {
		t.Fatalf("start=%d want 2", got[0].Start)
	}
}
