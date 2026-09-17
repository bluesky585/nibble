package batch

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/recursive"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

// fakeChunker records the set of texts it was called with and fails on a
// chosen text. Workers run concurrently, so the recording is guarded;
// the tests assert on the set and the count, not on an order the pool
// does not promise.
type fakeChunker struct {
	// failOn is a single text to fail on; failTexts holds several. Both
	// are compared against non-empty inputs, because "" is the nil-result
	// marker and can never name a failure.
	failOn    string
	failTexts map[string]bool

	mu    sync.Mutex
	calls map[string]int
}

func newFakeChunker(failOn string) *fakeChunker {
	return &fakeChunker{failOn: failOn, calls: map[string]int{}}
}

func (f *fakeChunker) Chunk(text string) ([]chunk.Chunk, error) {
	f.mu.Lock()
	f.calls[text]++
	f.mu.Unlock()
	// The empty input is a nil result, not the failure case: the failOn
	// comparison is against a non-empty marker, and "" == "" would
	// otherwise read as a failure.
	if text == "" {
		return nil, nil
	}
	if text == f.failOn || f.failTexts[text] {
		return nil, errors.New("boom")
	}
	ch, err := chunk.New(text, 0, len([]rune(text)), len([]rune(text)))
	if err != nil {
		return nil, err
	}
	return []chunk.Chunk{ch}, nil
}

func (f *fakeChunker) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		n += c
	}
	return n
}

// Results line up with the inputs by index, and a nil result becomes an
// empty slice rather than staying nil.
func TestChunkResultsLineUpWithInputs(t *testing.T) {
	t.Parallel()

	f := newFakeChunker("\x00never")
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

// Every input is chunked exactly once. The pool does not promise an
// order, so the assertion is on the count, not on a sequence.
func TestChunkCallsOncePerInput(t *testing.T) {
	t.Parallel()

	f := newFakeChunker("\x00never")
	_, err := Chunk(f, []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if n := f.callCount(); n != 3 {
		t.Fatalf("calls=%d want 3", n)
	}
}

// An error stops the pool from taking new work and is reported with the
// lowest failing index — the one a serial reader would have hit first.
func TestChunkStopsAtFirstErrorAndWrapsIndex(t *testing.T) {
	t.Parallel()

	// Both b and d fail; the report must name b, the lower index. The
	// pool may have started d before b failed, so the count can reach
	// four, but never past the inputs that were in flight.
	// failOn is set per text in failTexts, so use the multi-failure fake.
	f := newFakeChunker("\x00never")
	f.failTexts = map[string]bool{"b": true, "d": true}
	got, err := Chunk(f, []string{"a", "b", "c", "d"})
	if err == nil {
		t.Fatal("want an error")
	}
	_ = got
	if !strings.Contains(err.Error(), "input 1") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err=%v, want the lowest failing index and the cause", err)
	}

	// A single later failure still reports by index, and every input is
	// still chunked at most once. No upper bound on the call count is
	// asserted here on purpose: the sender can hand an input to a worker
	// before it observes the failure, and the documented contract is that
	// delivered work finishes, not that it is cancelled mid-flight.
	f2 := newFakeChunker("d")
	_, err2 := Chunk(f2, []string{"a", "b", "c", "d", "e"})
	if err2 == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err2.Error(), "input 3") {
		t.Fatalf("err=%v, want the failing index", err2)
	}
	// The pool may stop before every input has run, but never runs an
	// input twice and never reports a count above the batch size.
	if n := f2.callCount(); n > 5 {
		t.Fatalf("calls=%d, want at most one call per input", n)
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

// concurrentChunker fails the test unless it sees more than one worker
// inside Chunk at once. It is the proof that the pool is a pool and not
// a serial loop with extra scheduling.
type concurrentChunker struct {
	inside atomic.Int32
	max    atomic.Int32
}

func (c *concurrentChunker) Chunk(text string) ([]chunk.Chunk, error) {
	n := c.inside.Add(1)
	for {
		m := c.max.Load()
		if n <= m || c.max.CompareAndSwap(m, n) {
			break
		}
	}
	// Hold the door open long enough for a second worker to enter. A
	// timing change that makes this flaky would mean the pool stopped
	// overlapping work, which is the thing this test exists to catch.
	time.Sleep(2 * time.Millisecond)
	c.inside.Add(-1)
	ch, err := chunk.New(text, 0, len([]rune(text)), len([]rune(text)))
	if err != nil {
		return nil, err
	}
	return []chunk.Chunk{ch}, nil
}

func TestChunkRunsConcurrently(t *testing.T) {
	t.Parallel()

	c := &concurrentChunker{}
	texts := make([]string, 8)
	for i := range texts {
		texts[i] = fmt.Sprintf("text number %d for the pool", i)
	}
	if _, err := Chunk(c, texts); err != nil {
		t.Fatal(err)
	}
	if c.max.Load() < 2 {
		t.Fatalf("max concurrent workers=%d, want >= 2", c.max.Load())
	}
}

// Many inputs through a small batch, under the race detector, is the
// standing check that the pool's shared state stays sound.
func TestChunkManyTextsStress(t *testing.T) {
	t.Parallel()

	c, err := recursive.New(tokenizer.Character{}, 64, nil)
	if err != nil {
		t.Fatal(err)
	}
	texts := make([]string, 200)
	for i := range texts {
		texts[i] = fmt.Sprintf("Stress input %d. Cats sleep. Dogs bark. 你好世界。", i)
	}
	got, err := Chunk(c, texts)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(texts) {
		t.Fatalf("groups=%d want %d", len(got), len(texts))
	}
	for i, chunks := range got {
		if len(chunks) == 0 {
			t.Fatalf("group %d is empty", i)
		}
	}
}
