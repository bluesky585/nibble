package markdown

import (
	"testing"
	"unicode/utf8"
)

func assertSpan(t *testing.T, original string, tab Table) {
	t.Helper()
	runes := []rune(original)
	if tab.Start < 0 || tab.End > len(runes) || tab.Start > tab.End {
		t.Fatalf("range [%d, %d) invalid for %d runes", tab.Start, tab.End, len(runes))
	}
	if string(runes[tab.Start:tab.End]) != tab.Text {
		t.Fatalf("text %q != original[%d:%d]", tab.Text, tab.Start, tab.End)
	}
}

func TestTablesNone(t *testing.T) {
	t.Parallel()

	if got := Tables(""); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
	if got := Tables("no table here\n"); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestTablesOne(t *testing.T) {
	t.Parallel()

	original := "intro\n\n| h |\n| --- |\n| 1 |\n\noutro\n"
	got := Tables(original)
	if len(got) != 1 {
		t.Fatalf("len=%d", len(got))
	}
	assertSpan(t, original, got[0])
	if got[0].Text != "| h |\n| --- |\n| 1 |\n" {
		t.Fatalf("text=%q", got[0].Text)
	}
}

func TestTablesTwo(t *testing.T) {
	t.Parallel()

	original := "| a |\n| --- |\n| 1 |\n\n| b |\n| --- |\n| 2 |\n"
	got := Tables(original)
	if len(got) != 2 {
		t.Fatalf("len=%d got %+v", len(got), got)
	}
	assertSpan(t, original, got[0])
	assertSpan(t, original, got[1])
	if got[0].End > got[1].Start {
		t.Fatalf("overlap %+v %+v", got[0], got[1])
	}
}

func TestTablesUnicode(t *testing.T) {
	t.Parallel()

	original := "前\n| 列 |\n| --- |\n| 值 |\n"
	got := Tables(original)
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	assertSpan(t, original, got[0])
	if utf8.RuneCountInString(original[:3]) != 1 {
		t.Fatal("precondition: 前 is one rune, three bytes")
	}
	if got[0].Start != 2 {
		t.Fatalf("start=%d want 2 (after 前 and newline)", got[0].Start)
	}
}

func TestTablesNeedsSeparator(t *testing.T) {
	t.Parallel()

	if got := Tables("| not | a |\n| table |\n"); len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}
