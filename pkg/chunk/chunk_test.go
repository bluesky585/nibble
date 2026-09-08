package chunk

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		text       string
		start      int
		end        int
		tokenCount int
		wantErr    string
	}{
		{
			name:       "ascii",
			text:       "hello",
			start:      0,
			end:        5,
			tokenCount: 5,
		},
		{
			name:       "empty",
			text:       "",
			start:      3,
			end:        3,
			tokenCount: 0,
		},
		{
			name:       "unicode runes not bytes",
			text:       "你好",
			start:      0,
			end:        2,
			tokenCount: 2,
		},
		{
			name:    "negative start",
			text:    "a",
			start:   -1,
			end:     0,
			wantErr: "start must be >= 0",
		},
		{
			name:    "end before start",
			text:    "a",
			start:   2,
			end:     1,
			wantErr: "end must be >= start",
		},
		{
			name:       "negative token count",
			text:       "a",
			start:      0,
			end:        1,
			tokenCount: -1,
			wantErr:    "token_count must be >= 0",
		},
		{
			name:    "rune count mismatch uses bytes as end",
			text:    "你好",
			start:   0,
			end:     6,
			wantErr: "text rune count 2 must equal end-start 6",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := New(tt.text, tt.start, tt.end, tt.tokenCount)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Text != tt.text || got.Start != tt.start || got.End != tt.end || got.TokenCount != tt.tokenCount {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestChunkJSON(t *testing.T) {
	t.Parallel()

	c, err := New("hi", 0, 2, 2)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}

	var got Chunk
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got != c {
		t.Fatalf("round trip: got %+v want %+v", got, c)
	}
}
