package tokenizer

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestBigramSplit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text string
		want []string
	}{
		{text: "", want: nil},
		// Latin runs stay whole, whitespace runs pass through.
		{text: "hello world", want: []string{"hello", " ", "world"}},
		// A CJK run is tiled into non-overlapping pairs, odd tail stays whole.
		{text: "大模型", want: []string{"大模", "型"}},
		// A one-character run cannot form a bigram and stays a unigram.
		{text: "猫", want: []string{"猫"}},
		// Bigrams never cross a script boundary.
		{text: "大模型RAG", want: []string{"大模", "型", "RAG"}},
		// Whitespace breaks a run and passes through as its own piece.
		{text: "你好 世界", want: []string{"你好", " ", "世界"}},
		// Mixed scripts alternate correctly.
		{text: "猫cat犬", want: []string{"猫", "cat", "犬"}},
		// Punctuation breaks a run.
		{text: "你好，世界", want: []string{"你好", "，", "世界"}},
		// Kana and hangul are CJK scripts too.
		{text: "かな", want: []string{"かな"}},
		{text: "한국", want: []string{"한국"}},
		// Repeated characters still tile.
		{text: "人人人", want: []string{"人人", "人"}},
	}

	tok := Bigram{}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.text), func(t *testing.T) {
			t.Parallel()

			got := tok.Split(tt.text)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Split(%q)=%q want %q", tt.text, got, tt.want)
			}
			if tok.Count(tt.text) != len(got) {
				t.Fatalf("Count=%d len(Split)=%d", tok.Count(tt.text), len(got))
			}
			if Join(got) != tt.text {
				t.Fatalf("Join(Split(%q))=%q", tt.text, Join(got))
			}
		})
	}
}

// Split's tiling and Terms' sliding segmentation agree on scale: for a
// CJK run of n characters, Split yields ceil(n/2) pieces and Terms
// yields n-1 terms — both proportional to n, so a chunk budgeted in
// bigram tokens bounds its retrieval terms. Word would report one
// token for the whole run and bound nothing.
func TestBigramScaleMatchesTerms(t *testing.T) {
	t.Parallel()

	for _, n := range []int{2, 3, 5, 8, 21} {
		run := []rune("大模型检索增强生成技术今天天气很好很好很好很好很好很好很好很好很好")[:n]
		text := string(run)
		pieces := len(Bigram{}.Split(text))
		terms := len(Terms(text))
		wantPieces := (n + 1) / 2
		if pieces != wantPieces {
			t.Fatalf("run of %d: Split pieces = %d, want %d", n, pieces, wantPieces)
		}
		if terms != n-1 {
			t.Fatalf("run of %d: Terms = %d, want %d", n, terms, n-1)
		}
		if terms > 2*pieces {
			t.Fatalf("run of %d: terms %d outrun pieces %d", n, terms, pieces)
		}
	}

	// Bigrams never cross a script boundary in either segmentation.
	b := Bigram{}
	if got := b.Split("大模型RAG"); !reflect.DeepEqual(got, []string{"大模", "型", "RAG"}) {
		t.Fatalf("Split mixed = %q", got)
	}
	if got := Terms("大模型RAG"); !reflect.DeepEqual(got, []string{"大模", "模型", "rag"}) {
		t.Fatalf("Terms mixed = %q", got)
	}
}

// Split on pathological input still concatenates back to the original.
func TestBigramRoundTripFuzz(t *testing.T) {
	t.Parallel()

	alphabet := []rune("ab猫犬かな01 ，。")
	for i := 0; i < 2000; i++ {
		var b strings.Builder
		n := i % 40
		for j := 0; j < n; j++ {
			b.WriteRune(alphabet[(i*7+j*13)%len(alphabet)])
		}
		text := b.String()
		if got := Join(Bigram{}.Split(text)); got != text {
			t.Fatalf("Join(Split(%q))=%q", text, got)
		}
	}
}
