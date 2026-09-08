package split

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func texts(pieces []Piece) []string {
	out := make([]string, len(pieces))
	for i, p := range pieces {
		out[i] = p.Text
	}
	return out
}

func assertPieces(t *testing.T, original string, pieces []Piece) {
	t.Helper()
	var b strings.Builder
	runes := []rune(original)
	for i, p := range pieces {
		b.WriteString(p.Text)
		if p.Start < 0 || p.End > len(runes) || p.Start > p.End {
			t.Fatalf("piece %d range [%d, %d) invalid for %d runes", i, p.Start, p.End, len(runes))
		}
		if string(runes[p.Start:p.End]) != p.Text {
			t.Fatalf("piece %d text %q != original[%d:%d]", i, p.Text, p.Start, p.End)
		}
		if i > 0 && pieces[i-1].End != p.Start {
			t.Fatalf("gap at %d: prev end %d start %d", i, pieces[i-1].End, p.Start)
		}
	}
	if b.String() != original {
		t.Fatalf("join=%q want %q", b.String(), original)
	}
	if len(pieces) > 0 {
		if pieces[0].Start != 0 || pieces[len(pieces)-1].End != utf8.RuneCountInString(original) {
			t.Fatalf("pieces do not cover original: first=%d last=%d n=%d",
				pieces[0].Start, pieces[len(pieces)-1].End, utf8.RuneCountInString(original))
		}
	}
}

func TestTextAttach(t *testing.T) {
	t.Parallel()

	original := "a.b.c"
	tests := []struct {
		name   string
		attach Attach
		want   []string
	}{
		{name: "prev", attach: AttachPrev, want: []string{"a.", "b.", "c"}},
		{name: "next", attach: AttachNext, want: []string{"a", ".b", ".c"}},
		{name: "own", attach: AttachOwn, want: []string{"a", ".", "b", ".", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Text(original, Options{Delimiters: []string{"."}, Attach: tt.attach})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(texts(got), "|") != strings.Join(tt.want, "|") {
				t.Fatalf("got %q want %q", texts(got), tt.want)
			}
			assertPieces(t, original, got)
		})
	}
}

func TestTextConsecutiveDelims(t *testing.T) {
	t.Parallel()

	original := "a..b"
	got, err := Text(original, Options{Delimiters: []string{"."}, Attach: AttachPrev})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a.", ".", "b"}
	if strings.Join(texts(got), "|") != strings.Join(want, "|") {
		t.Fatalf("got %q want %q", texts(got), want)
	}
	assertPieces(t, original, got)
}

func TestTextLongestDelimiter(t *testing.T) {
	t.Parallel()

	original := "hello\n\nworld\nend"
	got, err := Text(original, Options{
		Delimiters: []string{"\n", "\n\n"},
		Attach:     AttachPrev,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"hello\n\n", "world\n", "end"}
	if strings.Join(texts(got), "|") != strings.Join(want, "|") {
		t.Fatalf("got %q want %q", texts(got), want)
	}
	assertPieces(t, original, got)
}

func TestTextUnicode(t *testing.T) {
	t.Parallel()

	original := "你好。世界"
	got, err := Text(original, Options{Delimiters: []string{"。"}, Attach: AttachPrev})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Text != "你好。" || got[1].Text != "世界" {
		t.Fatalf("got %+v", got)
	}
	if got[0].End != 3 || got[1].Start != 3 || got[1].End != 5 {
		t.Fatalf("rune offsets %+v %+v", got[0], got[1])
	}
	assertPieces(t, original, got)
}

func TestTextMinRunes(t *testing.T) {
	t.Parallel()

	original := "a.b.c.d"
	got, err := Text(original, Options{
		Delimiters: []string{"."},
		Attach:     AttachPrev,
		MinRunes:   4,
	})
	if err != nil {
		t.Fatal(err)
	}
	// "a." (2) + "b." (2) => "a.b." (4), then "c." + "d" => "c.d"
	want := []string{"a.b.", "c.d"}
	if strings.Join(texts(got), "|") != strings.Join(want, "|") {
		t.Fatalf("got %q want %q", texts(got), want)
	}
	assertPieces(t, original, got)
}

func TestTextNoDelimiter(t *testing.T) {
	t.Parallel()

	original := "hello"
	got, err := Text(original, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Text != original {
		t.Fatalf("got %+v", got)
	}
	assertPieces(t, original, got)
}

func TestTextEmpty(t *testing.T) {
	t.Parallel()

	got, err := Text("", Options{Delimiters: []string{"."}})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("got %+v", got)
	}
}

func TestTextErrors(t *testing.T) {
	t.Parallel()

	_, err := Text("a", Options{Delimiters: []string{""}})
	if err == nil {
		t.Fatal("expected empty delimiter error")
	}
	_, err = Text("a", Options{MinRunes: -1})
	if err == nil {
		t.Fatal("expected MinRunes error")
	}
}
