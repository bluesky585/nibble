package chunkerfuzz

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/codechunker"
	"github.com/bluesky585/nibble/pkg/embed"
	"github.com/bluesky585/nibble/pkg/markdownchunker"
	"github.com/bluesky585/nibble/pkg/recursive"
	"github.com/bluesky585/nibble/pkg/semantic"
	"github.com/bluesky585/nibble/pkg/sentencechunker"
	"github.com/bluesky585/nibble/pkg/tablechunker"
	"github.com/bluesky585/nibble/pkg/tokenchunker"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

// alphabets seeds every target in this package. They are the shapes that
// broke an earlier version while nibble was being written or that a chunker
// has to handle: characters outside the Basic Multilingual Plane (four bytes,
// and a BPE token can end inside one), zero-width-joiner emoji, CRLF, a run
// with no delimiter to split on, the Markdown shapes the markdown and table
// chunkers parse, an unparseable Go file, control bytes, and the empty input.
// They stay short on purpose; the fuzzer mutates from here.
var alphabets = []string{
	"Hello world. The quick brown fox jumps over the lazy dog.\n",
	"你好世界。这是一个测试。没有空格的长句也要切分。\n",
	"日本語のテキストとEnglish mixed. ひらがな、カタカナ、漢字。\n",
	"emoji 🙂 and a family 👨👩👧 here, plus a flag 🇨🇳.\n",
	"naïve café — dash, ellipsis… “quotes”, ﬁ ligature.\n",
	"𠀋𠀋𠀋 four-byte han repeated 𠀋.\n",
	"\r\nCRLF line\r\n\r\nblank line above\r\n",
	"| a | b |\n| --- | --- |\n| 1 | 2 |\n| 3 | 4 |\n",
	"| wide | row |\n| --- | --- |\n| " + strings.Repeat("x", 80) + " | 2 |\n",
	"# Heading\n\nProse.\n\n```go\nfunc main() { _ = 1 }\n```\n\nMore prose.\n",
	"~~~\ntilde fence\n~~~\n",
	"```\nunclosed fence\n",
	"| not a table\n| --- |\n",
	"package main\n\n// Doc.\nfunc main() {}\n\ntype T struct{ A int }\n",
	"not go source at all {{{ )))\n",
	// Python exercises the line scanner rather than a parser: a decorator and
	// its comment, a class whose methods are indented and so are not cuts, a
	// docstring holding a def that is not one, a def nested in an if, and a
	// bracket and a backslash continuation. prose and a fenced python block
	// make the markdown chunker pick Python rules for the fence.
	"import os\n\n\n@cache\n# Doc.\ndef f(a, b=[\n    1,\n]):\n    \"\"\"A docstring with a def fake(): in it.\"\"\"\n    return a + \\\n        b\n\n\nclass C:\n    def m(self):\n        if True:\n            def nested():\n                pass\n        return 1\n",
	"prose\n\n```python\ndef f():\n    pass\n```\n\nprose\n",
	"tabs\tand  doubled  spaces\n",
	strings.Repeat("token ", 40) + "\n",
	strings.Repeat("你", 120) + "\n",
	"\x00 nul, \x7f del\n",
	"",
	"a",
}

// seeds registers the corpus with a target. Every target in this package
// takes the same two arguments, so one helper serves them all: the input
// text, and a number folded into a size (see clampSize).
func seeds(f *testing.F) {
	for _, a := range alphabets {
		f.Add([]byte(a), 8)
	}
	// The alphabets joined reach past a single piece, so a split at one level
	// leaves a piece that still has to be split by the next.
	long := []byte(strings.Join(alphabets, ""))
	for _, size := range []int{1, 2, 3, 7, 16} {
		f.Add(long, size)
	}
}

// clampSize folds the fuzzer's number into a small budget. The budget is
// fuzzed because 1 is the size where everything collapses at once: single
// rune pieces, empty pieces from a token that ends inside a character, and a
// piece that is over budget on its own. Offsets must line up at any size.
func clampSize(n int) int {
	return 1 + int(uint(n)%16)
}

// maxInput is the longest text a target passes to a chunker. The fuzzer
// grows its input without bound, and a chunker that is linear in the input is
// still slow at a hundred thousand runes; one such input would hold a worker
// for minutes and look like a hang. Every seed is far below this, so the cap
// only trims what the fuzzer grew on its own, and a failure at the cap can be
// replayed from the minimizer's output like any other.
const maxInput = 4096

// trim cuts text to maxInput runes. It runs after sanitize, so it cuts on a
// rune boundary and the result stays valid UTF-8.
func trim(text string) string {
	if len(text) <= maxInput {
		return text
	}
	runes := []rune(text)
	if len(runes) <= maxInput {
		return text
	}
	return string(runes[:maxInput])
}

// sanitize keeps an input to what a caller can pass: a UTF-8 file or a JSON
// body. Invalid UTF-8 can come from neither, and replacing it lets the
// fuzzer spend its budget on text rather than on the replacement path.
func sanitize(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	return strings.ToValidUTF8(string(b), "�")
}

// ranges renders chunk offsets compactly for a failure message.
func ranges(chunks []chunk.Chunk) string {
	parts := make([]string, len(chunks))
	for i, c := range chunks {
		parts[i] = fmt.Sprintf("[%d,%d)", c.Start, c.End)
	}
	return strings.Join(parts, " ")
}

// chunker is the shape every chunker has.
type chunker interface {
	Chunk(text string) ([]chunk.Chunk, error)
}

// budget says how a chunker bounds the chunks it returns.
type budget int

const (
	// budgetTrue means no chunk holds more than size tokens, counted by the
	// ruler that splits the text. This is the promise a size limit makes,
	// and it is a promise about the text, not about a number the chunker
	// happens to write down: chunk.TokenCount is the chunker's own tally,
	// and a tally is not the same measurement as a recount. A chunker packs
	// pieces and adds up the piece counts; the same text, measured again
	// whole, can count differently.
	//
	// Under a ruler that counts every rune the two agree exactly, because
	// the pieces are runs of the text and the tally is their total length.
	// Under a word ruler the recount can differ either way: a delimiter run
	// that held a piece split off merges back, and a piece like ": " splits
	// apart, so the recount can come out below or above the tally. It has
	// not been observed above it, but that is a fuzzer's finding rather than
	// a proof, so a counterexample here would be a real one and not a
	// shortage of assertion.
	budgetTrue budget = iota

	// budgetReported means the recount is not a bound the chunker can honor:
	// only the chunker's own number is checked, and it must not claim more
	// than the budget it was given. Used for a BPE ruler, where rejoining
	// pieces at a window boundary can split or merge tokens, and the recount
	// has been measured to overshoot the budget by up to two tokens.
	budgetReported

	// budgetNone means the chunker exceeds size on purpose: markdown
	// reattaches a fence line to a boundary chunk, and a table row has to
	// carry its header, so neither can fit a small budget.
	budgetNone
)

// subject is one chunker under test, with the ruler that measures it and the
// budget it claims, so a check knows what to assert about it.
type subject struct {
	name   string
	chunk  chunker
	ruler  tokenizer.Tokenizer
	budget budget
	size   int
}

// ctors is every chunker that measures with a tokenizer, under the name the
// CLI uses. fast is absent: it takes no tokenizer and its size is bytes, so
// it is checked on its own terms.
var ctors = []struct {
	name   string
	build  func(tok tokenizer.Tokenizer, size int) (chunker, error)
	budget budget
}{
	{"recursive", func(tok tokenizer.Tokenizer, size int) (chunker, error) {
		return recursive.New(tok, size, nil)
	}, budgetTrue},
	{"sentence", func(tok tokenizer.Tokenizer, size int) (chunker, error) {
		return sentencechunker.New(tok, size, nil)
	}, budgetTrue},
	{"token", func(tok tokenizer.Tokenizer, size int) (chunker, error) {
		return tokenchunker.New(tok, size, 0)
	}, budgetTrue},
	{"table", func(tok tokenizer.Tokenizer, size int) (chunker, error) {
		return tablechunker.New(tok, size)
	}, budgetNone},
	{"code", func(tok tokenizer.Tokenizer, size int) (chunker, error) {
		return codechunker.New(tok, size)
	}, budgetTrue},
	{"markdown", func(tok tokenizer.Tokenizer, size int) (chunker, error) {
		return markdownchunker.New(tok, size)
	}, budgetNone},
	{"semantic", func(tok tokenizer.Tokenizer, size int) (chunker, error) {
		return semantic.New(tok, embed.Hashing{}, size, 0)
	}, budgetTrue},
}

// ruler pairs a tokenizer with the one property the budget rule has to know
// about it: whether its tokens are runs of the text.
//
// character and word both are, and they disagree about what a token is, so a
// chunker is checked under each: the rune ruler counts out exactly the
// offsets, the word ruler merges runs across some of the boundaries a
// chunker cuts at. Neither needs a vocabulary, which is why rulers below
// holds only those two and a single run of FuzzStandardChunkers covers every
// chunker.
//
// bpe marks a ruler whose tokens are not runs of the text — a BPE token can
// span several characters' bytes, or end inside one — which is what makes a
// recount a different measurement. tiktoken_test.go builds one.
type ruler struct {
	name string
	tok  tokenizer.Tokenizer
	bpe  bool
}

var rulers = []ruler{
	{"character", tokenizer.Character{}, false},
	{"word", tokenizer.Word{}, false},
}

// withRuler builds every chunker at one size under one ruler.
func withRuler(r ruler, size int) ([]subject, error) {
	var out []subject
	for _, c := range ctors {
		b := c.budget
		if r.bpe && b == budgetTrue {
			b = budgetReported
		}
		ch, err := c.build(r.tok, size)
		if err != nil {
			return nil, fmt.Errorf("build %s/%s at size %d: %w", r.name, c.name, size, err)
		}
		out = append(out, subject{
			name:   r.name + "/" + c.name,
			chunk:  ch,
			ruler:  r.tok,
			budget: b,
			size:   size,
		})
	}
	return out, nil
}

// standard builds every chunker at one size, once per vocabulary-free ruler.
func standard(size int) ([]subject, error) {
	var out []subject
	for _, r := range rulers {
		s, err := withRuler(r, size)
		if err != nil {
			return nil, err
		}
		out = append(out, s...)
	}
	return out, nil
}
