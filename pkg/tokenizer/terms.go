package tokenizer

import (
	"strings"
	"unicode"
)

// Terms splits text into index terms: the unit the hashing embedder
// hashes and a sparse scorer counts. Latin and digit runs come through
// whole, lowercased. A run of Han, kana, or hangul has no written word
// boundaries to split on, so it is segmented into sliding bigrams — the
// unit Lucene's CJK analyzers use, which needs no dictionary and keeps
// adjacent characters of a paraphrased query matching the document. A
// one-character run cannot form a bigram and stays a unigram. Bigrams
// never cross a script boundary: "大模型RAG" yields the bigrams of
// "大模型" plus the term "rag", not "型R", which is noise that would
// inflate document length without improving recall.
//
// Terms is the shared core of the bigram tokenizer's Split and of the
// sparse side's WordTerms; both callers need the same segmentation, or
// the chunk scale and the retrieval signals drift apart.
func Terms(text string) []string {
	var terms []string
	var latin strings.Builder
	var cjk []rune

	flushLatin := func() {
		if latin.Len() > 0 {
			terms = append(terms, latin.String())
			latin.Reset()
		}
	}
	flushCJK := func() {
		switch len(cjk) {
		case 0:
		case 1:
			terms = append(terms, string(cjk))
		default:
			for i := 0; i+1 < len(cjk); i++ {
				terms = append(terms, string(cjk[i:i+2]))
			}
		}
		cjk = cjk[:0]
	}

	for _, r := range strings.ToLower(text) {
		switch {
		case IsCJK(r):
			flushLatin()
			cjk = append(cjk, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			flushCJK()
			latin.WriteRune(r)
		default:
			flushLatin()
			flushCJK()
		}
	}
	flushLatin()
	flushCJK()
	return terms
}

// IsCJK reports whether the rune belongs to a script written without
// word boundaries, and so needs bigram segmentation: Han (Chinese),
// kana (Japanese syllabaries), and hangul (Korean).
func IsCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r)
}
