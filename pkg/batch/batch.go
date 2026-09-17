// Package batch chunks many texts through one chunker.
package batch

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/bluesky585/nibble/pkg/chunk"
)

// Chunker is what Chunk batches over. Every chunker package's Chunker
// satisfies it; so does buildchunk.Chunker, which is how the CLI and the
// HTTP API call this.
type Chunker interface {
	Chunk(text string) ([]chunk.Chunk, error)
}

// maxWorkers caps the pool. Four cores make three workers, so the
// machine keeps one core for everything else; past eight the returns
// from more workers flatten out while the scheduling cost does not.
func maxWorkers() int {
	n := runtime.NumCPU() * 3 / 4
	if n < 1 {
		n = 1
	}
	if n > 8 {
		n = 8
	}
	return n
}

// Chunk chunks each text with c and reports one result per input. The
// results line up with the inputs by index, and nil results become empty
// slices, so a caller that stores results by position never sees a nil it
// did not ask for.
//
// Texts run concurrently through a worker pool sized to the machine, so
// a directory walk costs one chunker's worth of time instead of one text
// after another. Chunkers in this module are safe for that: their Chunk
// methods keep no state between calls.
//
// On an error the pool stops taking new work, finishes what is running,
// and reports the lowest-index error wrapped with its input's index. The
// error path returns no partial batch: a caller that receives an error
// cannot tell which results are trustworthy without re-deriving the
// cutoff itself, and the lowest-index error is the one a serial reader
// would have hit first.
func Chunk(c Chunker, texts []string) ([][]chunk.Chunk, error) {
	out := make([][]chunk.Chunk, len(texts))
	if len(texts) == 0 {
		return out, nil
	}

	errMu := sync.Mutex{}
	firstErrIdx := -1
	var firstErr error
	// closeOnce guards done: two workers can fail at the same time, and
	// the check-then-close pattern is a race between them.
	var closeOnce sync.Once

	// jobs carries indexes; the workers read their text from texts so the
	// channel stays pointer- and string-header-sized.
	jobs := make(chan int)
	var wg sync.WaitGroup
	workers := maxWorkers()
	if workers > len(texts) {
		workers = len(texts)
	}
	done := make(chan struct{})
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				chunks, err := c.Chunk(texts[i])
				if err == nil && chunks == nil {
					chunks = []chunk.Chunk{}
				}
				out[i] = chunks // each worker writes its own index only
				if err != nil {
					errMu.Lock()
					if firstErrIdx < 0 || i < firstErrIdx {
						firstErrIdx, firstErr = i, err
					}
					errMu.Unlock()
					// One signal is enough; the sender stops and the
					// remaining queued indexes are dropped.
					closeOnce.Do(func() { close(done) })
				}
			}
		}()
	}

send:
	for i := range texts {
		select {
		case <-done:
			break send
		default:
		}
		select {
		case jobs <- i:
		case <-done:
			break send
		}
	}
	close(jobs)
	wg.Wait()

	if firstErr != nil {
		return nil, fmt.Errorf("input %d: %w", firstErrIdx, firstErr)
	}
	return out, nil
}
