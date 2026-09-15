package chunkerfuzz

import (
	"testing"

	"github.com/bluesky585/nibble/pkg/tokenizer/tiktoken"
)

// FuzzBPEChunkers drives the chunkers with a real BPE vocabulary. This is
// the configuration the rune-boundary rule exists for: a token can end
// inside a character, so a naive token window would cut a character in half
// and produce a Chunk whose text disagrees with its offsets. What is checked
// is the same as FuzzStandardChunkers — the chunks still reconstruct the
// input — plus two things only this vocabulary can check:
//
//   - Count(text) == len(Split(text)) for the tokenizer itself, at any text,
//     including one whose last character straddles a token boundary
//   - the chunker's own tally is never over the budget (see budgetReported;
//     the recount is deliberately not asserted, because rejoining pieces at
//     a boundary is a different measurement that overshoots by up to two
//     tokens on inputs like the ones below)
//
// The vocabulary is downloaded on a cold cache. When it cannot be fetched
// the target skips rather than fails, so an offline run reports nothing; the
// rune-boundary logic is covered without a network by splitOnRunes's own
// tests, and everything but this file needs no vocabulary at all.
func FuzzBPEChunkers(f *testing.F) {
	seeds(f)
	// Four-byte characters (never one token) and emoji (a token split across
	// several bytes, including pieces that are not valid UTF-8 alone) are
	// where a boundary lands inside a character.
	f.Add([]byte("𠀋𠀋𠀋𠀋"), 2)
	f.Add([]byte("🙂🙂🙂🙂"), 1)
	f.Add([]byte("你好世界。这是一个测试。"), 3)
	f.Add([]byte("\t\n|-#`~ \r"), 1)

	tok, err := tiktoken.New("")
	if err != nil {
		f.Skipf("tiktoken encoding unavailable (offline?): %v", err)
	}

	f.Fuzz(func(t *testing.T, data []byte, n int) {
		text := trim(sanitize(data))

		// The tokenizer's own contract, which the chunkers rely on: one
		// piece per token, and joining them restores the input.
		parts := tok.Split(text)
		if got, want := tok.Count(text), len(parts); got != want {
			t.Fatalf("Count=%d len(Split)=%d for %q", got, want, text)
		}
		if joined := joinAll(parts); joined != text {
			t.Fatalf("Split pieces do not restore %q: got %q", text, joined)
		}

		subjects, err := withRuler(ruler{name: "tiktoken", tok: tok, bpe: true}, clampSize(n))
		if err != nil {
			t.Fatalf("build chunkers: %v", err)
		}
		for _, s := range subjects {
			chunks, err := s.chunk.Chunk(text)
			if err != nil {
				t.Fatalf("%s returned an error for %q: %v", s.name, text, err)
			}
			checkChunks(t, s, text, chunks)
		}
	})
}

// joinAll concatenates the pieces a tokenizer returned. It is spelled out
// rather than using tokenizer.Join so a failure here is not attributed to
// the helper under test.
func joinAll(parts []string) string {
	var b []byte
	for _, p := range parts {
		b = append(b, p...)
	}
	return string(b)
}
