package tiktoken

import (
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

// newOrSkip loads the default encoding, skipping when the vocabulary cannot
// be fetched. The table is downloaded on a cold cache, so a test run without
// network should report nothing rather than fail; the rune-boundary tests
// above cover the logic that does not need a vocabulary.
func newOrSkip(t *testing.T) Tiktoken {
	t.Helper()
	tok, err := New("")
	if err != nil {
		t.Skipf("tiktoken encoding unavailable: %v", err)
	}
	return tok
}

// The rune-boundary rule is the part that can be wrong independently of the
// vocabulary, so it is tested without loading an encoding table.
func TestSplitOnRunes(t *testing.T) {
	t.Parallel()

	// One CJK character is three bytes, so a token ending inside one has no
	// whole rune left and must yield an empty piece.
	tests := []struct {
		name  string
		text  string
		sizes []int
		want  []string
	}{
		{"whole runes", "你好", []int{3, 3}, []string{"你", "好"}},
		{"mid character", "你好", []int{2, 4}, []string{"", "你好"}},
		// Boundaries at 1, 2, 3, 6 bytes: the third completes the first rune.
		{"three splits inside one rune", "你好", []int{1, 1, 1, 3}, []string{"", "", "你", "好"}},
		{"ascii", "hi!", []int{1, 2}, []string{"h", "i!"}},
		{"single token covers all", "你好", []int{6}, []string{"你好"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := tt.text
			got := splitOnRunes(text, tt.sizes)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d pieces want %d: %q", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("piece %d = %q want %q (all %q)", i, got[i], tt.want[i], got)
				}
			}
			if joined := strings.Join(got, ""); joined != text {
				t.Fatalf("pieces do not restore the input: %q", joined)
			}
			for i, p := range got {
				if !utf8.ValidString(p) {
					t.Fatalf("piece %d is not valid UTF-8: %q", i, p)
				}
			}
		})
	}
}

// A 4-byte character split across tokens must still come out whole.
func TestSplitOnRunesFourByte(t *testing.T) {
	t.Parallel()

	text := "a𠀋b" // "𠀋" is 4 bytes
	got := splitOnRunes(text, []int{1, 1, 3, 1})
	if joined := strings.Join(got, ""); joined != text {
		t.Fatalf("got %q want %q", joined, text)
	}
	for i, p := range got {
		if !utf8.ValidString(p) {
			t.Fatalf("piece %d invalid: %q", i, p)
		}
	}
	// The 4-byte rune is completed by the token that carries its last byte.
	if got[2] != "𠀋" {
		t.Fatalf("piece 2 = %q want the whole rune", got[2])
	}
}

// The real encoding: Count must equal len(Split), the pieces must restore
// the input, and each piece must be valid UTF-8. This loads the vocabulary
// (downloading it on a cold cache).
func TestTiktokenSplitInvariants(t *testing.T) {
	t.Parallel()

	tok := newOrSkip(t)
	if tok.Encoding() != DefaultEncoding {
		t.Fatalf("encoding=%q want %q", tok.Encoding(), DefaultEncoding)
	}

	texts := []string{
		"",
		"a",
		"Hello world.",
		"你好世界",
		"日本語のテキスト",
		"emoji 🙂 and a family 👨‍👩‍👧",
		"naïve café — punctuation, mixed.",
		"𠀋 rare han",
		strings.Repeat("token ", 100),
	}
	for _, text := range texts {
		parts := tok.Split(text)
		if got, want := tok.Count(text), len(parts); got != want {
			t.Fatalf("Count=%d len(Split)=%d for %q", got, want, text)
		}
		if joined := strings.Join(parts, ""); joined != text {
			t.Fatalf("pieces do not restore %q: got %q", text, joined)
		}
		for i, p := range parts {
			if !utf8.ValidString(p) {
				t.Fatalf("piece %d of %q is not valid UTF-8: %q", i, text, p)
			}
		}
	}
}

// A real token budget should differ from a rune count, which is the whole
// point of this tokenizer.
func TestTiktokenCountsDifferFromRunes(t *testing.T) {
	t.Parallel()

	tok := newOrSkip(t)
	text := "Hello world."
	if tok.Count(text) != 3 {
		t.Fatalf("cl100k should count %q as 3 tokens, got %d", text, tok.Count(text))
	}
	if utf8.RuneCountInString(text) == tok.Count(text) {
		t.Fatal("tokenizer agrees with a rune count; it is not measuring tokens")
	}
}

// A four-token budget must still cover a text of many more runes.
func TestTiktokenSplitCountsMatchBudget(t *testing.T) {
	t.Parallel()

	tok := newOrSkip(t)
	text := "The quick brown fox jumps over the lazy dog."
	parts := tok.Split(text)
	if len(parts) < 5 {
		t.Fatalf("expected a multi-token split, got %d pieces: %q", len(parts), parts)
	}
	if utf8.RuneCountInString(text) == len(parts) {
		t.Fatal("piecing per rune, not per token")
	}
}

// Constructing this tokenizer must not repeat the expensive load. It is
// built once per request by the HTTP API, so a per-call compile would be
// paid on every request.
func TestNewIsCheapAfterFirstLoad(t *testing.T) {
	t.Parallel()

	tok := newOrSkip(t)
	if _, err := New(""); err != nil {
		t.Fatal(err)
	}
	// 200 constructions must be far quicker than 200 cold loads, which would
	// take tens of seconds. One second is a generous ceiling that still
	// fails loudly if the handle stops being cached.
	start := time.Now()
	for i := 0; i < 200; i++ {
		if _, err := New(""); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("200 New calls took %v; the encoding handle is not cached", d)
	}
	// The cached handle must still work.
	if tok.Count("Hello world.") != 3 {
		t.Fatal("cached handle gave the wrong count")
	}
}

// Concurrent use of the cached handle must be safe; the HTTP API shares it
// across requests.
func TestConcurrentUse(t *testing.T) {
	t.Parallel()

	tok := newOrSkip(t)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			text := "你好 world 🙂"
			if tok.Count(text) != len(tok.Split(text)) {
				t.Error("Count != len(Split) under concurrency")
			}
		}()
	}
	wg.Wait()
}
