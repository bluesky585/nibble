package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bluesky585/nibble/internal/assertchunk"
	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/embed"
	"github.com/bluesky585/nibble/pkg/store"
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

func TestSearchAfterIndex(t *testing.T) {
	t.Parallel()

	api := New()
	h := api.Handler()

	idx := httptest.NewRecorder()
	h.ServeHTTP(idx, httptest.NewRequest(
		http.MethodPost,
		"/v1/index",
		strings.NewReader(`{"text":"cats sleep. quantum field.","chunker":"sentence","size":64}`),
	))
	if idx.Code != http.StatusOK {
		t.Fatalf("index status %d body=%s", idx.Code, idx.Body.String())
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost,
		"/v1/search",
		strings.NewReader(`{"query":"cats","k":1}`),
	))
	if rec.Code != http.StatusOK {
		t.Fatalf("search status %d body=%s", rec.Code, rec.Body.String())
	}

	var resp searchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Hits) != 1 || !strings.Contains(resp.Hits[0].Record.Chunk.Text, "cats") {
		t.Fatalf("got %+v", resp.Hits)
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/search", strings.NewReader(`{"query":""}`))
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
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

// Recursive accepts overlap: the next chunk repeats the previous
// chunk's tail, the way a token window widens.
func TestChunkRecursiveOverlap(t *testing.T) {
	t.Parallel()

	original := strings.Repeat("word ", 8)
	body, err := json.Marshal(map[string]any{
		"text": original, "chunker": "recursive", "size": 8, "overlap": 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chunk", bytes.NewReader(body))
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Chunks []chunk.Chunk `json:"chunks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Chunks) < 2 {
		t.Fatalf("chunks=%d, want several", len(resp.Chunks))
	}
	for i := 1; i < len(resp.Chunks); i++ {
		if resp.Chunks[i].Start != resp.Chunks[i-1].End-2 {
			t.Fatalf("chunk %d start=%d want %d", i, resp.Chunks[i].Start, resp.Chunks[i-1].End-2)
		}
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

// lang reaches the code chunker over HTTP the same way -lang does on the CLI.
// A bad language is reported rather than ignored, so a client cannot believe
// it selected Python while the server detected something else.
func TestChunkLang(t *testing.T) {
	t.Parallel()

	original := "def a():\n    pass\n\n\ndef b():\n    pass\n"
	raw, _ := json.Marshal(chunkRequest{Text: original, Chunker: "code", Lang: "python", Size: 24})
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
	if len(resp.Chunks) != 2 {
		t.Fatalf("len=%d want 2: %+v", len(resp.Chunks), resp.Chunks)
	}

	raw, _ = json.Marshal(chunkRequest{Text: original, Chunker: "code", Lang: "rust", Size: 24})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/chunk", bytes.NewReader(raw))
	Handler().ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("an unknown lang must not be accepted: %s", rec.Body.String())
	}
}

// The search scoring modes match the CLI's -scoring: dense (default),
// bm25 (term overlap, no embedder), hybrid (a blend). Scores across the
// modes are not comparable, so bm25 is checked on its ranking and its
// positivity, not against the dense numbers.
func TestSearchScoringModes(t *testing.T) {
	t.Parallel()

	api := New()
	h := api.Handler()

	idx := httptest.NewRecorder()
	h.ServeHTTP(idx, httptest.NewRequest(
		http.MethodPost,
		"/v1/index",
		// Size 24 keeps each sentence its own chunk, so the rare term lives
		// in one record instead of being fragmented by token fallback or
		// swallowed by a whole-document chunk.
		strings.NewReader(`{"text":"cats sleep on mats. quarks feel the strong force. dogs bark loudly.","chunker":"sentence","size":24}`),
	))
	if idx.Code != http.StatusOK {
		t.Fatalf("index status %d body=%s", idx.Code, idx.Body.String())
	}

	// bm25 without an embedder: the embedder field is ignored, and the
	// rare term beats the common one.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost,
		"/v1/search",
		strings.NewReader(`{"query":"quarks","k":1,"scoring":"bm25"}`),
	))
	if rec.Code != http.StatusOK {
		t.Fatalf("bm25 status %d body=%s", rec.Code, rec.Body.String())
	}
	var resp searchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Hits) != 1 || !strings.Contains(resp.Hits[0].Record.Chunk.Text, "quarks") {
		t.Fatalf("bm25 got %+v", resp.Hits)
	}
	if resp.Hits[0].Score <= 0 {
		t.Fatalf("bm25 score should be positive: %+v", resp.Hits[0])
	}

	// hybrid with the default weight finds the obvious sentence.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost,
		"/v1/search",
		strings.NewReader(`{"query":"cats sleep","k":1,"scoring":"hybrid"}`),
	))
	if rec.Code != http.StatusOK {
		t.Fatalf("hybrid status %d body=%s", rec.Code, rec.Body.String())
	}
	resp = searchResponse{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Hits) != 1 || !strings.Contains(resp.Hits[0].Record.Chunk.Text, "cats") {
		t.Fatalf("hybrid got %+v", resp.Hits)
	}

	// An unknown mode is a usage error, not a silent dense.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost,
		"/v1/search",
		strings.NewReader(`{"query":"cats","scoring":"splade"}`),
	))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "scoring") {
		t.Fatalf("bad scoring: status %d body=%s", rec.Code, rec.Body.String())
	}
}

// A hybrid weight outside [0, 1] is rejected the way the CLI rejects it.
func TestSearchHybridWeightRange(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/search", strings.NewReader(
		`{"query":"cats","scoring":"hybrid","hybrid_weight":2}`))
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "weight") {
		t.Fatalf("status %d body=%s", rec.Code, rec.Body.String())
	}
}

// A persistent API backs its index with a SQLite file: records survive
// the process, so a restart on the same path resumes from what the last
// process indexed. The memory API loses everything on exit, which is
// the default it stays.
func TestPersistentAPIResumes(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "api.db")

	// First process: index and stop.
	{
		api, err := NewPersistent(path)
		if err != nil {
			t.Fatal(err)
		}
		h := api.Handler()
		idx := httptest.NewRecorder()
		h.ServeHTTP(idx, httptest.NewRequest(
			http.MethodPost,
			"/v1/index",
			strings.NewReader(`{"text":"cats sleep on mats. dogs bark all night.","chunker":"sentence","size":24}`),
		))
		if idx.Code != http.StatusOK {
			t.Fatalf("index status %d body=%s", idx.Code, idx.Body.String())
		}
		if err := api.Close(); err != nil {
			t.Fatal(err)
		}
	}

	// Second process: the file is the index, and search finds it.
	{
		api, err := NewPersistent(path)
		if err != nil {
			t.Fatal(err)
		}
		defer api.Close()
		h := api.Handler()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(
			http.MethodPost,
			"/v1/search",
			strings.NewReader(`{"query":"cats","k":1}`),
		))
		if rec.Code != http.StatusOK {
			t.Fatalf("search status %d body=%s", rec.Code, rec.Body.String())
		}
		var resp searchResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if len(resp.Hits) != 1 || !strings.Contains(resp.Hits[0].Record.Chunk.Text, "cats") {
			t.Fatalf("got %+v", resp.Hits)
		}
	}
}

// A persistent API refuses to open a path it cannot use, instead of
// falling back to memory and pretending everything is fine.
func TestPersistentAPIBadPath(t *testing.T) {
	t.Parallel()

	if _, err := NewPersistent(""); err == nil {
		t.Fatal("empty path accepted")
	}
}

// A source labels where indexed chunks came from. It rides the index
// request, shows up on the hits, lists with a count, and a delete
// removes exactly that origin's chunks.
func TestSourceLifecycle(t *testing.T) {
	t.Parallel()

	api := New()
	h := api.Handler()

	index := func(text, source string) {
		t.Helper()
		body, err := json.Marshal(map[string]any{
			"text": text, "chunker": "sentence", "size": 64, "source": source,
		})
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/index", bytes.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("index status %d body=%s", rec.Code, rec.Body.String())
		}
	}
	index("cats sleep on mats.", "a.md")
	index("dogs bark all night.", "a.md")
	index("birds migrate in autumn.", "b.md")

	// The list holds both origins with their counts.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/sources", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("sources status %d", rec.Code)
	}
	var srcs struct {
		Sources []store.Source `json:"sources"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &srcs); err != nil {
		t.Fatal(err)
	}
	if len(srcs.Sources) != 2 || srcs.Sources[0].Name != "a.md" || srcs.Sources[0].Count != 2 {
		t.Fatalf("sources=%+v", srcs.Sources)
	}

	// Deleting b.md removes one record; a.md survives.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/v1/sources/b.md", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status %d", rec.Code)
	}
	var del struct {
		Deleted int `json:"deleted"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &del); err != nil {
		t.Fatal(err)
	}
	if del.Deleted != 1 {
		t.Fatalf("deleted=%d, want 1", del.Deleted)
	}

	// A second delete of the same source reports 0 rather than an error.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/v1/sources/b.md", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("second delete status %d", rec.Code)
	}

	// The remaining index still searches, and the hit carries its source.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(
		http.MethodPost, "/v1/search", strings.NewReader(`{"query":"cats","k":5}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("search status %d", rec.Code)
	}
	var resp searchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Hits) != 2 || resp.Hits[0].Record.Source != "a.md" {
		t.Fatalf("hits=%+v", resp.Hits)
	}
}

// An index request without a source labels its chunks with the empty
// source, which lists and deletes like any other name.
func TestSourceEmpty(t *testing.T) {
	t.Parallel()

	api := New()
	h := api.Handler()
	idx := httptest.NewRecorder()
	h.ServeHTTP(idx, httptest.NewRequest(
		http.MethodPost, "/v1/index",
		strings.NewReader(`{"text":"cats sleep. dogs bark.","chunker":"sentence","size":64}`)))
	if idx.Code != http.StatusOK {
		t.Fatalf("index status %d", idx.Code)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/sources", nil))
	var srcs struct {
		Sources []store.Source `json:"sources"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &srcs); err != nil {
		t.Fatal(err)
	}
	if len(srcs.Sources) != 1 || srcs.Sources[0].Name != "" || srcs.Sources[0].Count != 1 {
		t.Fatalf("sources=%+v", srcs.Sources)
	}
}

// An empty index lists an empty source set, not null.
func TestSourcesEmptyIndex(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	New().Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/sources", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"sources": []`) {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

// A search with "source" only sees that origin's records, and k counts
// filtered hits. The field is a pointer on the wire: absent searches
// everything, an empty string filters to the records indexed without a
// source.
func TestSearchSourceFilter(t *testing.T) {
	t.Parallel()

	api := New()
	h := api.Handler()
	index := func(text, source string) {
		t.Helper()
		body, err := json.Marshal(map[string]any{
			"text": text, "chunker": "sentence", "size": 64, "source": source,
		})
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/index", bytes.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("index status %d", rec.Code)
		}
	}
	index("cats sleep on warm mats. cats purr loudly.", "a.md")
	index("dogs bark all night long. dogs howl too.", "b.md")

	search := func(body string) searchResponse {
		t.Helper()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/search", strings.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("search status %d body=%s", rec.Code, rec.Body.String())
		}
		var resp searchResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		return resp
	}

	// Filtered to a.md, every hit mentions cats.
	got := search(`{"query":"cats","k":5,"source":"a.md"}`)
	if len(got.Hits) == 0 {
		t.Fatal("no hits for a.md")
	}
	for _, hit := range got.Hits {
		if hit.Record.Source != "a.md" {
			t.Fatalf("hit from %q leaked through the a.md filter", hit.Record.Source)
		}
	}

	// The empty string filters to records indexed without a source —
	// here, none — while an absent source searches everything.
	if got := search(`{"query":"cats","k":5,"source":""}`); len(got.Hits) != 0 {
		t.Fatalf("empty source got %d hits, want 0", len(got.Hits))
	}
	if got := search(`{"query":"cats","k":5}`); len(got.Hits) == 0 {
		t.Fatal("absent source found nothing")
	}

	// An unknown source is empty, not an error.
	if got := search(`{"query":"cats","k":5,"source":"nope"}`); len(got.Hits) != 0 {
		t.Fatalf("nope got %d hits, want 0", len(got.Hits))
	}
}
