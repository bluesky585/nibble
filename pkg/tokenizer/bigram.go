package tokenizer

import "unicode"

// Bigram measures CJK text by its retrieval unit. Latin and digit runs
// are one token each, the way Word groups them; a run of Han, kana, or
// hangul is segmented into sliding bigrams — the same unit Terms uses
// for the embedding and BM25 sides. Word, splitting on whitespace
// alone, counts an entire CJK sentence as one token, so a size budget
// measured in it is meaningless for CJK text; Bigram exists so the
// chunker's ruler and the retriever's terms agree on that script.
type Bigram struct{}

// Split returns each Latin and digit run whole, the whitespace runs
// between them, and each CJK run tiled into non-overlapping pairs (an
// odd trailing character stays a unigram). Pieces concatenate to the
// original text, as the Tokenizer contract requires — which is why the
// pairs do not overlap here even though the retrieval terms of Terms
// do: a tokenizer's pieces must reassemble, and sliding bigrams
// cannot.
func (Bigram) Split(text string) []string {
	if text == "" {
		return nil
	}

	var parts []string
	var b []rune
	lastCJK := false
	lastSpace := false
	started := false

	flush := func() {
		if len(b) == 0 {
			return
		}
		if lastCJK {
			for i := 0; i+1 < len(b); i += 2 {
				parts = append(parts, string(b[i:i+2]))
			}
			if len(b)%2 == 1 {
				parts = append(parts, string(b[len(b)-1]))
			}
		} else {
			parts = append(parts, string(b))
		}
		b = b[:0]
	}

	for _, r := range text {
		space := unicode.IsSpace(r)
		cjk := IsCJK(r)
		switch {
		case !started:
			lastSpace, lastCJK, started = space, cjk, true
		case cjk != lastCJK || space != lastSpace:
			flush()
			lastSpace, lastCJK = space, cjk
		}
		b = append(b, r)
	}
	flush()
	return parts
}

// Count returns the number of pieces Split yields.
func (t Bigram) Count(text string) int {
	return len(t.Split(text))
}
