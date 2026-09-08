package tokenizer

import (
	"fmt"
	"reflect"
	"testing"
	"unicode/utf8"
)

func TestCharacterRoundTrip(t *testing.T) {
	t.Parallel()

	tok := Character{}
	tests := []string{
		"",
		"hello",
		"你好",
		"👍",
		"Go 1.26",
	}

	for _, text := range tests {
		t.Run(fmt.Sprintf("%q", text), func(t *testing.T) {
			t.Parallel()

			ids := tok.Encode(text)
			if tok.Count(text) != len(ids) {
				t.Fatalf("Count=%d len(Encode)=%d", tok.Count(text), len(ids))
			}
			if tok.Count(text) != utf8.RuneCountInString(text) {
				t.Fatalf("Count=%d rune count=%d", tok.Count(text), utf8.RuneCountInString(text))
			}
			got := tok.Decode(ids)
			if got != text {
				t.Fatalf("Decode(Encode(%q)) = %q", text, got)
			}
			parts := tok.Split(text)
			if tok.Count(text) != len(parts) {
				t.Fatalf("Count=%d len(Split)=%d", tok.Count(text), len(parts))
			}
			if Join(parts) != text {
				t.Fatalf("Join(Split(%q)) = %q", text, Join(parts))
			}
		})
	}
}

func TestCharacterEncodeIDs(t *testing.T) {
	t.Parallel()

	got := Character{}.Encode("ab你")
	want := []int{'a', 'b', '你'}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestCharacterCountNotBytes(t *testing.T) {
	t.Parallel()

	text := "你好"
	got := Character{}.Count(text)
	if got != 2 {
		t.Fatalf("Count(%q)=%d, want 2 runes (not %d bytes)", text, got, len(text))
	}
}
