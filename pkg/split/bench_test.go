package split

import (
	"testing"

	"github.com/bluesky585/nibble/internal/corpus"
)

// The delimiters match sentencechunker.DefaultDelimiters. They are spelled
// out here because the package cannot import that one: sentencechunker
// imports split, and a test import would close the cycle.
var benchDelims = []string{"。", "！", "？", ".", "!", "?"}

func BenchmarkSplitSentence(b *testing.B) {
	text := corpus.Prose()
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		if _, err := Text(text, Options{Delimiters: benchDelims, Attach: AttachPrev}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSplitSentenceCJK(b *testing.B) {
	text := "你好。世界！今天天气不错。猫在睡觉。狗在叫。鸟在唱歌。" +
		"这是一段没有分隔符但是比较长的文字用来测量扫描器在没有切点时的表现"
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		if _, err := Text(text, Options{Delimiters: benchDelims, Attach: AttachPrev}); err != nil {
			b.Fatal(err)
		}
	}
}
