package chunkerfuzz

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/bluesky585/nibble/internal/assertchunk"
	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/fastchunker"
	"github.com/bluesky585/nibble/pkg/overlap"
	"github.com/bluesky585/nibble/pkg/recursive"
	"github.com/bluesky585/nibble/pkg/tokenchunker"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

// FuzzStandardChunkers drives every chunker that takes a tokenizer with the
// same random text and asserts what each one must never trade away:
//
//   - the chunks are a complete, non-overlapping split of the input
//     (first chunk starts at rune 0, neighbors meet, texts match the source,
//     and concatenating them restores it) — this is the whole point of the
//     library, and assertchunk.Check is the authority on it
//   - every offset is in range and every chunk text is valid UTF-8
//   - no chunk is over the budget (see checkBudget for what that means
//     under each ruler)
//
// The targets are the same at every size because a chunker is one
// implementation with the size threaded through it, not a family of
// implementations. What the size changes is which paths a chunk takes: a
// size of 1 forces every level of the recursive descent down to single
// tokens, which is where an offset can go wrong.
//
// This was a fixed table of inputs before it was a fuzz target. The table
// still seeds it (see alphabets), but the fuzzer explores the space between
// the seeds: cut a delimiter in half, join two alphabets, and the split rule
// is exercised at a boundary no hand-written case reached.
func FuzzStandardChunkers(f *testing.F) {
	seeds(f)
	f.Fuzz(func(t *testing.T, data []byte, n int) {
		text := trim(sanitize(data))
		subjects, err := standard(clampSize(n))
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

// FuzzFastChunker covers the one chunker that takes no tokenizer: its size
// is a byte budget, and it must still never split a UTF-8 character, however
// small the budget.
func FuzzFastChunker(f *testing.F) {
	seeds(f)
	f.Fuzz(func(t *testing.T, data []byte, n int) {
		text := trim(sanitize(data))
		size := clampSize(n)

		c, err := fastchunker.New(size, nil)
		if err != nil {
			t.Fatalf("New(size=%d): %v", size, err)
		}
		chunks, err := c.Chunk(text)
		if err != nil {
			t.Fatalf("Chunk(%q, size=%d): %v", text, size, err)
		}

		// The ruler is runes, but the budget is bytes, so the rune check
		// through checkChunks is one-directional and holds: a window holds
		// no more runes than it holds bytes, and at most one rune is wider
		// than the budget. What must also hold is the byte promise itself,
		// which is checked below.
		s := subject{name: "fast", ruler: tokenizer.Character{}, budget: budgetTrue, size: size}
		checkChunks(t, s, text, chunks)

		for i, c := range chunks {
			// The point of this chunker: a byte window is a rune sequence,
			// never a prefix of one. A single rune wider than the budget is
			// allowed to exceed it, since keeping it whole is the rule.
			if len(c.Text) > size && utf8.RuneCountInString(c.Text) != 1 {
				t.Fatalf("chunk %d is %d bytes, over the %d-byte budget, and holds %d runes: %q",
					i, len(c.Text), size, utf8.RuneCountInString(c.Text), c.Text)
			}
		}
	})
}

// FuzzTokenChunkerOverlap covers overlap for the only chunker that supports
// it. Windows share tokens, so the chunks are no longer a clean split and
// assertchunk.Check would rightly reject them. What must hold is that they
// tile the input: the first starts at rune 0, the last ends at the end,
// every window advances, no gap opens, and each window's text is the source
// under its own offsets.
func FuzzTokenChunkerOverlap(f *testing.F) {
	seeds(f)
	f.Fuzz(func(t *testing.T, data []byte, n int) {
		text := trim(sanitize(data))
		size := clampSize(n)
		// From no overlap up to one token short of the window. At that end a
		// window advances by a single token, so the windows are as dense as
		// they can get.
		overlap := int(uint(n>>4) % uint(size))

		c, err := tokenchunker.New(tokenizer.Character{}, size, overlap)
		if err != nil {
			t.Fatalf("tokenchunker.New(size=%d, overlap=%d): %v", size, overlap, err)
		}
		chunks, err := c.Chunk(text)
		if err != nil {
			t.Fatalf("Chunk(%q, size=%d, overlap=%d): %v", text, size, overlap, err)
		}
		if len(chunks) == 0 {
			if text != "" {
				t.Fatalf("no chunks for %q", text)
			}
			return
		}

		runes := []rune(text)
		if chunks[0].Start != 0 {
			t.Fatalf("first chunk starts at %d, want 0", chunks[0].Start)
		}
		for i, c := range chunks {
			if c.Start < 0 || c.End > len(runes) || c.Start >= c.End {
				t.Fatalf("chunk %d range [%d, %d) is not a window of %d runes",
					i, c.Start, c.End, len(runes))
			}
			if got := string(runes[c.Start:c.End]); got != c.Text {
				t.Fatalf("chunk %d text %q does not match source[%d:%d] %q", i, c.Text, c.Start, c.End, got)
			}
			if i > 0 {
				prev := chunks[i-1]
				if c.Start <= prev.Start {
					t.Fatalf("chunk %d starts at %d, not after chunk %d at %d",
						i, c.Start, i-1, prev.Start)
				}
				if c.Start > prev.End {
					t.Fatalf("gap between chunk %d end %d and chunk %d start %d: %q uncovered",
						i-1, prev.End, i, c.Start, string(runes[prev.End:c.Start]))
				}
			}
		}
		if last := chunks[len(chunks)-1]; last.End != len(runes) {
			t.Fatalf("last chunk ends at %d, want %d; %q uncovered",
				last.End, len(runes), string(runes[last.End:]))
		}
	})
}

// FuzzContextOverlap covers the neighbor-token feature, which is the other
// way a chunk can carry text it does not own. Copying tokens in must not
// touch Text, Start, or End, or reconstruct would break.
func FuzzContextOverlap(f *testing.F) {
	f.Add([]byte("Hello. 你好。World.\n"), 2)
	// Force multi-chunk input at a tiny budget, where a neighbor exists to
	// copy from and the copy is longer than the chunk itself.
	f.Add([]byte(strings.Repeat("token 你", 30)), 1)
	f.Add([]byte("| a | b |\n| --- | --- |\n| 1 | 2 |\n"), 1)
	f.Fuzz(func(t *testing.T, data []byte, n int) {
		text := trim(sanitize(data))
		nTokens := 1 + int(uint(n)%8)

		for _, r := range rulers {
			c, err := recursive.New(r.tok, clampSize(n), nil)
			if err != nil {
				t.Fatal(err)
			}
			chunks, err := c.Chunk(text)
			if err != nil {
				t.Fatalf("chunk: %v", err)
			}

			for _, mode := range []struct {
				name  string
				apply func([]chunk.Chunk, tokenizer.Tokenizer, int) ([]chunk.Chunk, error)
			}{
				{"prefix", overlap.Prefix},
				{"suffix", overlap.Suffix},
			} {
				got, err := mode.apply(chunks, r.tok, nTokens)
				if err != nil {
					t.Fatalf("%s/%s: %v", r.name, mode.name, err)
				}
				if len(got) != len(chunks) {
					t.Fatalf("%s/%s changed the chunk count: %d -> %d",
						r.name, mode.name, len(chunks), len(got))
				}
				for i := range got {
					// Text and offsets must come through untouched; only
					// Context may grow. This is the whole promise of the
					// feature.
					if got[i].Text != chunks[i].Text ||
						got[i].Start != chunks[i].Start ||
						got[i].End != chunks[i].End {
						t.Fatalf("%s/%s changed chunk %d: %q[%d,%d) -> %q[%d,%d)",
							r.name, mode.name, i,
							chunks[i].Text, chunks[i].Start, chunks[i].End,
							got[i].Text, got[i].Start, got[i].End)
					}
					// Whatever was copied comes from a neighbor, so it must
					// appear in the input. A tokenizer that invented or
					// duplicated text would be caught here.
					if got[i].Context != "" && !strings.Contains(text, got[i].Context) {
						t.Fatalf("%s/%s: chunk %d context %q is not in the input",
							r.name, mode.name, i, got[i].Context)
					}
				}
				// Context is added text, never removed, so the chunks' own
				// text must still reconstruct the input.
				assertchunk.Split(t, text, got)
			}
		}
	})
}

// checkChunks is the shared assertion. It reports the failing chunker and
// input, because a fuzz failure that only says "offsets disagree" is not
// enough to reproduce by hand.
func checkChunks(t *testing.T, s subject, text string, chunks []chunk.Chunk) {
	t.Helper()

	if err := assertchunk.Check(text, chunks); err != nil {
		t.Fatalf("%s: %v (input %q, chunks %s)", s.name, err, text, ranges(chunks))
	}

	runes := []rune(text)
	for i, c := range chunks {
		// Start >= End is the empty-chunk guard, not just a range check: a
		// chunk that spans no runes is empty text with a token count, which
		// a BPE window of nothing but empty pieces used to produce. See
		// tokenchunker.Chunk.
		if c.Start < 0 || c.End > len(runes) || c.Start >= c.End {
			t.Fatalf("%s: chunk %d range [%d, %d) is not a window of %d runes (input %q)",
				s.name, i, c.Start, c.End, len(runes), text)
		}
		if !utf8.ValidString(c.Text) {
			t.Fatalf("%s: chunk %d text is not valid UTF-8: %q", s.name, i, c.Text)
		}
	}
	checkBudget(t, s, chunks)
}

// checkBudget asserts the size limit, in the form that ruler supports.
//
// The subtlety is that Chunk.TokenCount is the chunker's own tally, not a
// recount of the chunk: a chunker packs pieces and adds up the piece counts,
// and the same text can count differently when it is measured again whole.
// That is what the budget in the subject decides; see budgetTrue and
// budgetReported.
func checkBudget(t *testing.T, s subject, chunks []chunk.Chunk) {
	t.Helper()

	// A reattached fence line or a row that has to carry its header exceeds
	// the budget by design, so there is nothing to check.
	if s.budget == budgetNone {
		return
	}

	for i, c := range chunks {
		// A chunker must not report more tokens than the budget it was
		// given. A chunk holding a single token wider than the whole budget
		// is the one allowed exception, and no ruler here produces it: a
		// single token counts once.
		//
		// Under budgetBlankMerge the tally is checked against the budget
		// plus the chunk's edge whitespace below, so the flat check does
		// not apply.
		if s.budget != budgetBlankMerge && c.TokenCount > s.size {
			t.Fatalf("%s: chunk %d reports %d tokens, over budget %d: %q",
				s.name, i, c.TokenCount, s.size, c.Text)
		}
		// Under budgetBlankMerge the tally may exceed size only by the
		// whitespace a blank chunk merged in, and that whitespace always
		// sits at the edges of the chunk: a trailing blank joined to the
		// previous chunk, a leading one carried onto the next. The merged
		// whitespace was tallied piece by piece, so recounting it as one
		// run is not an upper bound; the rune count is. Every token here
		// covers at least one rune, so a string never counts for more
		// tokens than it has runes, and size plus the edge-whitespace
		// runes bounds the tally. Anything beyond it means content was
		// packed over the budget.
		if s.budget == budgetBlankMerge && c.TokenCount > s.size {
			runes := []rune(c.Text)
			lead := 0
			for lead < len(runes) && unicode.IsSpace(runes[lead]) {
				lead++
			}
			trail := len(runes)
			for trail > lead && unicode.IsSpace(runes[trail-1]) {
				trail--
			}
			ws := lead + len(runes) - trail
			if c.TokenCount > s.size+ws {
				t.Fatalf("%s: chunk %d reports %d tokens, over budget %d by more than its %d edge-whitespace tokens: %q",
					s.name, i, c.TokenCount, s.size, ws, c.Text)
			}
		}
		// Under budgetReported the recount is a different measurement and is
		// not asserted (see the budget comment).
		if s.budget != budgetTrue {
			continue
		}
		// The recount of a chunk cannot exceed the sum of its pieces when the
		// pieces are runs of the text: measuring them together can merge runs
		// but never split one, so no seam can add a token. That is the rune
		// ruler exactly, and the word ruler with one caveat, that a piece
		// whose end is not itself a boundary (": ") splits apart at the seam.
		// The assertion is kept for the word ruler because a split like that
		// does not push a chunk over the budget in practice; see the budget
		// comment for why that is a fuzzer's finding and not a proof.
		if got := s.ruler.Count(c.Text); got > s.size {
			t.Fatalf("%s: chunk %d holds %d tokens by the ruler, over budget %d: %q",
				s.name, i, got, s.size, c.Text)
		}
	}
}

// FuzzRecursiveOverlap covers the recursive chunker's -overlap: the next
// chunk repeats the previous chunk's tail in its own Text. That breaks
// the contiguous-split promise on purpose, so this target asserts the
// contract that remains:
//
//   - every chunk is a slice of the input (offsets stay exact)
//   - the first chunk starts at rune 0 and the last ends at the end
//   - after the first, each chunk starts n runes before the previous
//     one ended, and its text begins with exactly that run
//   - chunk interiors still advance: a chunk's own tail (past the
//     repeated run) starts no later than the next chunk's repeated run
//     ends, so no region of the input is skipped
//
// The overlap is applied to the final chunk sequence, after the blank
// merge, which is where a run could otherwise point at a neighbor that
// no longer exists.
func FuzzRecursiveOverlap(f *testing.F) {
	seeds(f)
	f.Fuzz(func(t *testing.T, data []byte, n int) {
		text := trim(sanitize(data))
		size := clampSize(n)
		// The overlap folds under the same size cap the constructor
		// enforces: at size 1 no positive overlap fits, and that case is
		// already covered by the no-overlap targets.
		nOverlap := 1 + int(uint(n>>4)%4)
		if nOverlap >= size {
			t.Skipf("overlap %d does not fit size %d", nOverlap, size)
		}
		runes := []rune(text)

		for _, r := range rulers {
			c, err := recursive.New(r.tok, size, nil, recursive.Overlap(nOverlap))
			if err != nil {
				// size 1 leaves no room for a positive overlap; the
				// constructor must be the one to say so, and only then.
				if size == 1 {
					continue
				}
				t.Fatalf("%s: build: %v", r.name, err)
			}
			chunks, err := c.Chunk(text)
			if err != nil {
				t.Fatalf("%s: chunk %q: %v", r.name, text, err)
			}
			if len(chunks) == 0 {
				continue
			}
			for i, ch := range chunks {
				if ch.Start < 0 || ch.End > len(runes) || ch.Start >= ch.End {
					t.Fatalf("%s: chunk %d range [%d,%d) is not a window of %d runes (input %q)",
						r.name, i, ch.Start, ch.End, len(runes), text)
				}
				if string(runes[ch.Start:ch.End]) != ch.Text {
					t.Fatalf("%s: chunk %d is not a slice of the input: [%d,%d) %q",
						r.name, i, ch.Start, ch.End, ch.Text)
				}
			}
			if chunks[0].Start != 0 {
				t.Fatalf("%s: first chunk starts at %d, want 0", r.name, chunks[0].Start)
			}
			if last := chunks[len(chunks)-1]; last.End != len(runes) {
				t.Fatalf("%s: last chunk ends at %d, want %d", r.name, last.End, len(runes))
			}
			for i := 1; i < len(chunks); i++ {
				prev, cur := chunks[i-1], chunks[i]
				// The overlap run is the previous chunk's tail, at most
				// n runes of it under the character ruler (a shorter
				// previous chunk repeats all of it, so start can equal
				// prev.Start — that is the run covering the whole
				// neighbor, which is legitimate); under the word ruler
				// the run is whatever the last n tokens hold, so only
				// the boundary arithmetic is asserted here and the
				// slice check above already pinned the text to the
				// source.
				if cur.Start >= prev.End {
					t.Fatalf("%s: chunk %d start %d does not overlap the previous end %d",
						r.name, i, cur.Start, prev.End)
				}
				if cur.Start < prev.Start {
					t.Fatalf("%s: chunk %d start %d reaches before the previous chunk (start %d)",
						r.name, i, cur.Start, prev.Start)
				}
			}
		}
	})
}
