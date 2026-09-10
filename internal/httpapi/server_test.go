package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bluesky585/nibble/internal/assertchunk"
	"github.com/bluesky585/nibble/pkg/embed"
)

func TestHealth(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"ok": true`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestIndexStores(t *testing.T) {
	t.Parallel()

	api := New()
	body := `{"text":"cats sleep. quantum field.","chunker":"sentence","size":64}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/index", strings.NewReader(body))
	api.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}

	var resp indexResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Count < 1 {
		t.Fatalf("count=%d", resp.Count)
	}

	q, err := embed.Hashing{}.Embed([]string{"cats"})
	if err != nil {
		t.Fatal(err)
	}
	hits, err := api.mem.Search(q[0], 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || !strings.Contains(hits[0].Record.Chunk.Text, "cats") {
		t.Fatalf("got %+v", hits)
	}
}

func TestIndexEmpty(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/index", strings.NewReader(`{"text":""}`))
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"count": 0`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestChunkOK(t *testing.T) {
	t.Parallel()

	body := `{"text":"Hello. World.","chunker":"sentence","size":64}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chunk", strings.NewReader(body))
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}

	var resp chunkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, "Hello. World.", resp.Chunks)
}

func TestChunkUnicodeDefaults(t *testing.T) {
	t.Parallel()

	original := "你好。世界！"
	raw, _ := json.Marshal(chunkRequest{Text: original, Size: 3})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chunk", bytes.NewReader(raw))
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}

	var resp chunkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	assertchunk.Split(t, original, resp.Chunks)
	if resp.Chunks[0].End != 3 {
		t.Fatalf("want rune offset 3, got %+v", resp.Chunks[0])
	}
}

func TestChunkEmpty(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chunk", strings.NewReader(`{"text":""}`))
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}

	var resp chunkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Chunks == nil {
		t.Fatal("chunks should be empty list, not null")
	}
	assertchunk.Split(t, "", resp.Chunks)
}

func TestChunkBadEmbedder(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chunk", strings.NewReader(`{"text":"hi","embedder":"magic"}`))
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestChunkBadOptions(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chunk", strings.NewReader(`{"text":"hi","chunker":"magic"}`))
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestChunkOverlapRejected(t *testing.T) {
	t.Parallel()

	body := `{"text":"hi","chunker":"recursive","overlap":1}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chunk", strings.NewReader(body))
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestChunkMethodNotAllowed(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/chunk", nil)
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestTokenOverlap(t *testing.T) {
	t.Parallel()

	body := `{"text":"hello","chunker":"token","size":3,"overlap":1}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chunk", strings.NewReader(body))
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}

	var resp chunkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Chunks) != 2 || resp.Chunks[0].Text != "hel" || resp.Chunks[1].Text != "llo" {
		t.Fatalf("got %+v", resp.Chunks)
	}
}

func TestUnknownField(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chunk", strings.NewReader(`{"text":"hi","nope":1}`))
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}
