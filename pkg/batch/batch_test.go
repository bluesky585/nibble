package batch

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/recursive"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

// fakeChunker records the order of its calls and fails on a chosen text,
// so the tests can assert both the batching behavior and the error path.
type fakeChunker struct {
	failOn string
	calls  []string
}

func (f *fakeChunker) Chunk(text string) ([]chunk.Chunk, error) {
	f.calls = append(f.calls, text)
	// The empty input is a nil result, not the failure case: the failOn
	// comparison is against a non-empty marker, and "" == "" would
	// otherwise read as a failure.
	if text == "" {
		return nil, nil
	}
	if text == f.failOn {
		return nil, errors.New("boom")
	}
	ch, err := chunk.New(text, 0, len([]rune(text)), len([]rune(text)))
	if err != nil {
		return nil, err
	}
	return []chunk.Chunk{ch}, nil
}

// Results line up with the inputs by index, and a nil result becomes an
// empty slice rather than staying nil.
func TestChunkResultsLineUpWithInputs(t *testing.T) {
	t.Parallel()

	f := &fakeChunker{}
	got, err := Chunk(f, []string{"alpha", "", "gamma"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	if len(got[0]) != 1 || got[0][0].Text != "alpha" {
		t.Fatalf("got[0]=%+v", got[0])
	}
	if got[1] == nil || len(got[1]) != 0 {
		t.Fatalf("empty input should give an empty slice, got %#v", got[1])
	}
	if len(got[2]) != 1 || got[2][0].Text != "gamma" {
		t.Fatalf("got[2]=%+v", got[2])
	}
}

// Every input is chunked, in input order.
func TestChunkCallsInInputOrder(t *testing.T) {
	t.Parallel()

	f := &fakeChunker{}
	_, err := Chunk(f, []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(f.calls, ",") != "a,b,c" {
		t.Fatalf("calls=%v", f.calls)
	}
}

// The first error stops the run: inputs after the failing one are never
// chunked, and the error names the failing index.
func TestChunkStopsAtFirstErrorAndWrapsIndex(t *testing.T) {
	t.Parallel()

	f := &fakeChunker{failOn: "b"}
	got, err := Chunk(f, []string{"a", "b", "c", "d"})
	if err == nil {
		t.Fatal("want an error")
	}
	if got != nil {
		t.Fatalf("error path must not return partial results, got %d groups", len(got))
	}
	if !strings.Contains(err.Error(), "input 1") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err=%v, want the index and the cause", err)
	}
	if strings.Join(f.calls, ",") != "a,b" {
		t.Fatalf("calls after the failure: %v", f.calls)
	}
}

// An empty batch is an empty result, not an error.
func TestChunkEmptyBatch(t *testing.T) {
	t.Parallel()

	got, err := Chunk(&fakeChunker{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("len=%d want 0", len(got))
	}
}

// A real chunker through the same interface produces results identical
// to calling it one text at a time.
func TestChunkMatchesSequentialCalls(t *testing.T) {
	t.Parallel()

	texts := []string{
		"Cats sleep. Dogs bark. Birds sing.",
		"你好。世界！今天天气不错。",
		"# Heading\n\nProse.\n\n- a\n- b\n",
		"",
	}

	one := func(texts []string) [][]chunk.Chunk {
		t.Helper()
		var out [][]chunk.Chunk
		for _, text := range texts {
			c, err := recursive.New(tokenizer.Character{}, 20, nil)
			if err != nil {
				t.Fatal(err)
			}
			chunks, err := c.Chunk(text)
			if err != nil {
				t.Fatal(err)
			}
			if chunks == nil {
				chunks = []chunk.Chunk{}
			}
			out = append(out, chunks)
		}
		return out
	}

	c, err := recursive.New(tokenizer.Character{}, 20, nil)
	if err != nil {
		t.Fatal(err)
	}
	batched, err := Chunk(c, texts)
	if err != nil {
		t.Fatal(err)
	}
	want := one(texts)
	if len(batched) != len(want) {
		t.Fatalf("groups=%d want %d", len(batched), len(want))
	}
	for i := range want {
		if len(batched[i]) != len(want[i]) {
			t.Fatalf("group %d: len=%d want %d", i, len(batched[i]), len(want[i]))
		}
		for j := range want[i] {
			// Chunk holds a slice field, so == is not available; DeepEqual
			// on the whole struct is the point of the test.
			if !reflect.DeepEqual(batched[i][j], want[i][j]) {
				t.Fatalf("group %d chunk %d: %+v want %+v", i, j, batched[i][j], want[i][j])
			}
		}
	}
}
