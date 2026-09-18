package scoring

import (
	"reflect"
	"testing"

	"github.com/bluesky585/nibble/pkg/chunk"
)

// A run of Han characters becomes sliding bigrams: the term unit that
// stands in for the word boundaries Chinese does not write. Bigrams
// match a paraphrase because two adjacent characters of the query
// landing in the document is evidence, where the whole run is not.
func TestWordTermsCJKBigram(t *testing.T) {
	t.Parallel()

	got := WordTerms("检索增强生成")
	want := []string{"检索", "索增", "增强", "强生", "生成"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// A single Han character between delimiters cannot form a bigram, so
// it stays as a unigram term: dropping it would lose a real query
// word.
func TestWordTermsCJKShortRun(t *testing.T) {
	t.Parallel()

	got := WordTerms("好。")
	want := []string{"好"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// Han and Latin runs are segmented separately: a bigram that crosses
// the script boundary ("型R") is noise, not evidence, and inflates the
// document length without improving recall.
func TestWordTermsMixedScript(t *testing.T) {
	t.Parallel()

	got := WordTerms("大模型RAG检索")
	want := []string{"大模", "模型", "rag", "检索"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// Hiragana and katakana have no written word boundaries either, so
// they bigram the same way.
func TestWordTermsKana(t *testing.T) {
	t.Parallel()

	got := WordTerms("テキスト処理")
	want := []string{"テキ", "キス", "スト", "ト処", "処理"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// English terms come through whole, as they always have: this is the
// regression net for the Latin path.
func TestWordTermsEnglishUnchanged(t *testing.T) {
	t.Parallel()

	got := WordTerms("The cat, sat on the mat 42 times!")
	want := []string{"the", "cat", "sat", "on", "the", "mat", "42", "times"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// The real defect this fixes: a Chinese query that paraphrases the
// document, sharing words but not the exact character run, scored 0
// under the whole-run term splitter. Under bigrams the shared words
// score above zero, and the document holding more of the query's
// vocabulary ranks first.
func TestBM25CJKParaphrase(t *testing.T) {
	t.Parallel()

	docs := []chunk.Chunk{
		doc("本文介绍如何提高检索质量，包括分词和排序的策略。"),
		doc("Go 程序由包组成，包是编译和链接的单元。"),
	}
	bm := NewBM25(docs, WordTerms)

	if got := bm.Score(0, "如何提升检索的效果"); got <= 0 {
		t.Fatalf("paraphrased query scored %v, want above 0", got)
	}
	if got := bm.Score(1, "如何提升检索的效果"); got != 0 {
		t.Fatalf("unrelated document scored %v, want 0", got)
	}
}
