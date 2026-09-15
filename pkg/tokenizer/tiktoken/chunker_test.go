package tiktoken_test

import (
	"testing"

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
			// No window may exceed the budget by more than the runes of the
			// token that straddles a character boundary.
			for i, ch := range chunks {
				if ch.TokenCount > size {
					t.Fatalf("chunk %d has %d tokens, budget %d", i, ch.TokenCount, size)
				}
			}
		}
	}
}
