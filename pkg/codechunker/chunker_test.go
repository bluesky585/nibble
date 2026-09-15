package codechunker

import (
	"strings"
	"testing"

	"github.com/bluesky585/nibble/internal/assertchunk"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

func TestNewValidation(t *testing.T) {
	t.Parallel()

	_, err := New(nil, 8)
	if err == nil || !strings.Contains(err.Error(), "tokenizer is required") {
		t.Fatalf("err=%v", err)
	}
	_, err = New(tokenizer.Character{}, 0)
	if err == nil || !strings.Contains(err.Error(), "size must be > 0") {
		t.Fatalf("err=%v", err)
	}
}

func TestChunkPacksSmallFile(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 512)
	if err != nil {
		t.Fatal(err)
	}

	original := "package p\n\nfunc A() {}\n\nfunc B() {}\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) != 1 {
		t.Fatalf("len=%d want 1", len(got))
	}
}

func TestChunkSplitsFunctions(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 24)
	if err != nil {
		t.Fatal(err)
	}

	original := "package p\n\nfunc A() {}\n\nfunc B() {}\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) < 2 {
		t.Fatalf("expected funcs in separate chunks, got %+v", got)
	}
	joined := ""
	for _, ch := range got {
		joined += ch.Text
	}
	if !strings.Contains(joined, "func A()") || !strings.Contains(joined, "func B()") {
		t.Fatalf("missing funcs: %+v", got)
	}
}

func TestChunkKeepsDocCommentWithFunc(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 40)
	if err != nil {
		t.Fatal(err)
	}

	original := "package p\n\n// Hello does a thing.\nfunc Hello() {}\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)

	found := false
	for _, ch := range got {
		if strings.Contains(ch.Text, "Hello does a thing") && strings.Contains(ch.Text, "func Hello") {
			found = true
		}
	}
	if !found {
		t.Fatalf("doc comment should stay with the func: %+v", got)
	}
}

func TestChunkParseFallback(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 3)
	if err != nil {
		t.Fatal(err)
	}

	original := "not go source at all"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if got[0].Text != "not" {
		t.Fatalf("expected token fallback, got %+v", got)
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

func TestChunkUnicodeComment(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 512)
	if err != nil {
		t.Fatal(err)
	}

	original := "package p\n\n// 你好\nfunc A() {}\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if got[0].End != runeCount(original) {
		t.Fatalf("end=%d want %d", got[0].End, runeCount(original))
	}
}

func runeCount(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

func TestNewLanguageValidation(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"python", "py", "Python3", "go", "golang", ""} {
		if _, err := New(tokenizer.Character{}, 8, Language(name)); err != nil {
			t.Fatalf("Language(%q): %v", name, err)
		}
	}
	if _, err := New(tokenizer.Character{}, 8, Language("rust")); err == nil ||
		!strings.Contains(err.Error(), "unknown language") {
		t.Fatalf("err=%v", err)
	}
}

// Python has no parser in the standard library, so it is cut by a line
// scanner. A definition has to land in one piece with its body, its doc
// comment, and its decorators.
func TestChunkPythonDefinitionWithDecorator(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 40, Language("python"))
	if err != nil {
		t.Fatal(err)
	}
	original := "@cache\n# Returns a value.\ndef f():\n    return 1\n\n\ndef g():\n    return 2\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)

	found := false
	for _, ch := range got {
		if strings.Contains(ch.Text, "@cache") && strings.Contains(ch.Text, "Returns a value") &&
			strings.Contains(ch.Text, "def f()") {
			found = true
		}
	}
	if !found {
		t.Fatalf("decorator and comment must stay with the def: %+v", got)
	}
}

// A class body is indented, so its methods are not top-level definitions and
// must not be cut away from the class.
func TestChunkPythonClassStaysWhole(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 512, Language("python"))
	if err != nil {
		t.Fatal(err)
	}
	original := "class C:\n    def m(self):\n        return 1\n"
	got, err := c.Chunk(original)
	if err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, got)
	if len(got) != 1 {
		t.Fatalf("len=%d want 1: %+v", len(got), got)
	}
}

// Text that looks like a cut but is not: a def inside a string, a comment, a
// bracket, or a set of triple quotes. Cutting there would still reconstruct,
// but it would split a statement in half, so the scanner leaves it alone.
// Every case also has a real def at the end, so the count is "the false
// boundary was refused and the true one was taken", not just "no cut found".
//
// The scanner is asserted directly rather than through Chunk, because Chunk
// packs pieces back together when they fit the budget: at a size that holds
// the whole input, every case here would come back as one chunk whatever the
// scanner did. Each case still runs through Chunk and assertchunk.Split, so
// the pieces counted here are the ones Chunk is built on.
func TestPythonCutsAvoidFalseBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want int // pieces expected from the scanner
	}{
		{
			name: "def in a string literal",
			text: "s = \"def not real():\"\n\n\ndef real():\n    pass\n",
			want: 2,
		},
		{
			name: "def in a comment",
			text: "# def not real():\n\n\ndef real():\n    pass\n",
			want: 2,
		},
		{
			name: "def in a triple-quoted string",
			text: "s = \"\"\"\ndef not real():\n\"\"\"\n\n\ndef real():\n    pass\n",
			want: 2,
		},
		{
			name: "a call spanning brackets",
			text: "x = f(\n    a,\n    b,\n)\n\n\ndef real():\n    pass\n",
			want: 2,
		},
		{
			name: "a line continued by a backslash",
			text: "x = 1 + \\\n    2\n\n\ndef real():\n    pass\n",
			want: 2,
		},
		{
			name: "a def nested in an if",
			text: "if True:\n    def nested():\n        pass\n\n\ndef real():\n    pass\n",
			want: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pieces := pythonPieces(tt.text)
			if len(pieces) != tt.want {
				t.Fatalf("pieces=%d want %d: %+v", len(pieces), tt.want, pieces)
			}
			// The pieces must tile the input: a scanner that skipped text
			// would still pass a length check but break reconstruction.
			var joined strings.Builder
			at := 0
			for _, p := range pieces {
				if p.Start != at {
					t.Fatalf("piece starts at %d want %d: %+v", p.Start, at, pieces)
				}
				joined.WriteString(p.Text)
				at = p.End
			}
			if got := joined.String(); got != tt.text {
				t.Fatalf("pieces do not tile the input:\n got %q\nwant %q", got, tt.text)
			}

			c, err := New(tokenizer.Character{}, 512, Language("python"))
			if err != nil {
				t.Fatal(err)
			}
			chunks, err := c.Chunk(tt.text)
			if err != nil {
				t.Fatal(err)
			}
			assertchunk.Split(t, tt.text, chunks)
		})
	}
}

// The Python scorer is a line scanner, so it must never lose text or report
// an offset that disagrees with the source, whatever the input.
func TestChunkPythonReconstructs(t *testing.T) {
	t.Parallel()

	c, err := New(tokenizer.Character{}, 40, Language("python"))
	if err != nil {
		t.Fatal(err)
	}
	texts := []string{
		"",
		"\n\n\n",
		"x = 1",
		"x = 1\n",
		"def f():\n",
		"class A:\n\tdef m(self):\n\t\tpass\n",
		"@d\ndef f():\n    pass\n",
		"'''\nunterminated triple\n",
		"def f():  # trailing comment\n    pass\n",
		"s = 'it\\'s'\n",
		"def 名字():\n    return \"你好\"\n",
		"x = (\n",
	}
	for _, text := range texts {
		got, err := c.Chunk(text)
		if err != nil {
			t.Fatalf("Chunk(%q): %v", text, err)
		}
		assertchunk.Split(t, text, got)
	}
}
