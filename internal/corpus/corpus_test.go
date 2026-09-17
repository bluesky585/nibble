package corpus

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// The corpus doubles as a correctness fixture: if a benchmark input ever
// fails the contract the benchmarks still measure, they are measuring
// nothing. One cheap test keeps that from drifting.
func TestCorpusReconstructs(t *testing.T) {
	for name, text := range map[string]string{
		"prose":  Prose(),
		"mixed":  Mixed(),
		"go":     GoSource(),
		"python": PythonSource(),
	} {
		if text == "" {
			t.Fatalf("%s is empty", name)
		}
		if !utf8.ValidString(text) {
			t.Fatalf("%s is not valid UTF-8", name)
		}
	}
	if !strings.Contains(Mixed(), "|") {
		t.Fatal("mixed should hold a table")
	}
	if !strings.Contains(Mixed(), "捕猎") {
		t.Fatal("mixed should hold CJK")
	}
}
