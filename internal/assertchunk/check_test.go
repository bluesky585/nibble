package assertchunk

import (
	"strings"
	"testing"

	"github.com/bluesky585/nibble/pkg/chunk"
)

func mustChunk(t *testing.T, text string, start, end, tokenCount int) chunk.Chunk {
	t.Helper()
	c, err := chunk.New(text, start, end, tokenCount)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCheck(t *testing.T) {
	t.Parallel()

	hello := mustChunk(t, "hello", 0, 5, 5)
	nihao := mustChunk(t, "你好", 0, 2, 2)

	tests := []struct {
		name     string
		original string
		chunks   []chunk.Chunk
		wantErr  string
	}{
		{
			name:     "empty original",
			original: "",
			chunks:   nil,
		},
		{
			name:     "single chunk",
			original: "hello",
			chunks:   []chunk.Chunk{hello},
		},
		{
			name:     "two chunks",
			original: "hello",
			chunks: []chunk.Chunk{
				mustChunk(t, "he", 0, 2, 2),
				mustChunk(t, "llo", 2, 5, 3),
			},
		},
		{
			name:     "unicode runes",
			original: "你好",
			chunks:   []chunk.Chunk{nihao},
		},
		{
			name:     "empty list for non-empty original",
			original: "hello",
			chunks:   nil,
			wantErr:  "empty chunk list",
		},
		{
			name:     "first start not zero",
			original: "hello",
			chunks:   []chunk.Chunk{mustChunk(t, "ello", 1, 5, 4)},
			wantErr:  "first chunk start is 1, want 0",
		},
		{
			name:     "last end short",
			original: "hello",
			chunks:   []chunk.Chunk{mustChunk(t, "hel", 0, 3, 3)},
			wantErr:  "last chunk end is 3, want 5",
		},
		{
			name:     "gap",
			original: "hello",
			chunks: []chunk.Chunk{
				mustChunk(t, "he", 0, 2, 2),
				mustChunk(t, "lo", 3, 5, 2),
			},
			wantErr: "gap or overlap",
		},
		{
			name:     "text does not match original",
			original: "hello",
			chunks:   []chunk.Chunk{mustChunk(t, "world", 0, 5, 5)},
			wantErr:  "text does not match original",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := Check(tt.original, tt.chunks)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestSplitPasses(t *testing.T) {
	t.Parallel()

	original := "你好世界"
	chunks := []chunk.Chunk{
		mustChunk(t, "你好", 0, 2, 2),
		mustChunk(t, "世界", 2, 4, 2),
	}
	Split(t, original, chunks)
}
