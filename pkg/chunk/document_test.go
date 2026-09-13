package chunk

import (
	"encoding/json"
	"testing"
)

func TestNewDocumentJSON(t *testing.T) {
	t.Parallel()

	c, err := New("hi", 0, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	doc := NewDocument("a.txt", "hi", []Chunk{c})

	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var got Document
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Path != "a.txt" || got.Content != "hi" || len(got.Chunks) != 1 {
		t.Fatalf("got %+v", got)
	}
	if got.Chunks[0].Text != got.Content {
		t.Fatalf("chunk text %q != content", got.Chunks[0].Text)
	}
}

func TestNewDocumentNilChunks(t *testing.T) {
	t.Parallel()

	doc := NewDocument("", "", nil)
	if doc.Chunks == nil {
		t.Fatal("chunks should be empty list, not null")
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"content":"","chunks":[]}` {
		t.Fatalf("json=%s", raw)
	}
}
