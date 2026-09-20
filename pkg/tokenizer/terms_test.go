package tokenizer

import (
	"reflect"
	"testing"
)

func TestTerms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text string
		want []string
	}{
		{text: "", want: nil},
		// Latin and digit runs come through whole, lowercased.
		{text: "Hello World 123", want: []string{"hello", "world", "123"}},
		// Punctuation separates terms.
		{text: "name,color", want: []string{"name", "color"}},
		// A CJK run becomes sliding bigrams.
		{text: "大模型", want: []string{"大模", "模型"}},
		// A one-character run stays a unigram.
		{text: "猫", want: []string{"猫"}},
		// Bigrams never cross a script boundary.
		{text: "大模型RAG", want: []string{"大模", "模型", "rag"}},
		// Kana and hangul segment too.
		{text: "かな書き", want: []string{"かな", "な書", "書き"}},
		{text: "한국어", want: []string{"한국", "국어"}},
		// Punctuation separates CJK runs.
		{text: "你好，世界", want: []string{"你好", "世界"}},
	}
	for _, tt := range tests {
		if got := Terms(tt.text); !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("Terms(%q)=%q want %q", tt.text, got, tt.want)
		}
	}
}
