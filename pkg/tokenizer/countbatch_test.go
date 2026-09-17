package tokenizer

import (
	"reflect"
	"testing"
)

// fakeTokenizer records Count calls, so a test can prove batch use.
type fakeTokenizer struct {
	counts  int
	asCount map[string]int
}

func (f *fakeTokenizer) Split(text string) []string { return nil }
func (f *fakeTokenizer) Count(text string) int {
	f.counts++
	return f.asCount[text]
}

func TestCountBatchLinesUpWithCount(t *testing.T) {
	t.Parallel()

	tok := &fakeTokenizer{asCount: map[string]int{"a": 1, "bb": 2, "": 0}}
	texts := []string{"a", "bb", ""}
	got := CountBatch(tok, texts)
	if !reflect.DeepEqual(got, []int{1, 2, 0}) {
		t.Fatalf("got %v", got)
	}
	if tok.counts != len(texts) {
		t.Fatalf("Count called %d times, want %d", tok.counts, len(texts))
	}
}

func TestCountBatchEmpty(t *testing.T) {
	t.Parallel()

	tok := &fakeTokenizer{asCount: map[string]int{}}
	if got := CountBatch(tok, nil); len(got) != 0 {
		t.Fatalf("got %v, want empty", got)
	}
	if tok.counts != 0 {
		t.Fatalf("Count called %d times, want 0", tok.counts)
	}
}
