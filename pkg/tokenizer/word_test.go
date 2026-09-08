package tokenizer

import (
	"fmt"
	"reflect"
	"testing"
)

func TestWordSplit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text string
		want []string
	}{
		{text: "", want: nil},
		{text: "hello", want: []string{"hello"}},
		{text: "hello world", want: []string{"hello", " ", "world"}},
		{text: "hello  world", want: []string{"hello", "  ", "world"}},
		{text: "  a", want: []string{"  ", "a"}},
		{text: "a  ", want: []string{"a", "  "}},
		{text: "你好 世界", want: []string{"你好", " ", "世界"}},
		{text: "one\ntwo", want: []string{"one", "\n", "two"}},
	}

	tok := Word{}
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
