package tiktoken_test

import (
	"testing"
	"unicode/utf8"

	"github.com/bluesky585/nibble/internal/assertchunk"
	"github.com/bluesky585/nibble/pkg/tokenchunker"
	"github.com/bluesky585/nibble/pkg/tokenizer/tiktoken"
)

// A BPE token can end mid-character, so feeding this tokenizer to a chunker
// is what could break rune offsets. assertchunk.Split checks that the pieces
// concatenate to the input and that every offset matches, which is the
// property a real token budget must not trade away.
func TestTokenChunkerWithRealVocabulary(t *testing.T) {
	t.Parallel()

	tok, err := tiktoken.New("")
	if err != nil {
		t.Skipf("tiktoken encoding unavailable (offline?): %v", err)
	}

	texts := []string{
		"Hello world. The quick brown fox.",
		"你好世界。这是一个测试。",
		"日本語のテキストとEnglish mixed.",
		"emoji 🙂 and a family 👨‍👩‍👧 here",
		"𠀋𠀋𠀋 rare han repeated",
		"",
	}
	for _, text := range texts {
		for _, size := range []int{1, 2, 3, 5, 64} {
			c, err := tokenchunker.New(tok, size, 0)
			if err != nil {
				t.Fatalf("New(size=%d): %v", size, err)
			}
			chunks, err := c.Chunk(text)
			if err != nil {
				t.Fatalf("Chunk(%q, size=%d): %v", text, size, err)
			}
			assertchunk.Split(t, text, chunks)
			for i, ch := range chunks {
				// A character wider than the budget cannot be cut, so a
				// window holding one may exceed the budget. 𠀋 is three
				// tokens under cl100k_base, so at size 1 or 2 it does.
				// Beyond that, no window may exceed the budget.
				if ch.TokenCount > size && utf8.RuneCountInString(ch.Text) > size {
					t.Fatalf("chunk %d has %d tokens and %d runes, budget %d",
						i, ch.TokenCount, utf8.RuneCountInString(ch.Text), size)
				}
				// A window that is all empty pieces spans no runes. It used
				// to be emitted as an empty chunk with a non-empty token
				// count; it must be skipped instead. Every character wider
				// than the budget is what triggers it, so these sizes are
				// the ones that cover it.
				if ch.Text == "" || ch.Start >= ch.End {
					t.Fatalf("chunk %d is empty at size %d: %q [%d,%d) %d tokens",
						i, size, ch.Text, ch.Start, ch.End, ch.TokenCount)
				}
			}
		}
	}
}
