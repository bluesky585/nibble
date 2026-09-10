// Package httpapi serves chunking over HTTP.
package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/bluesky585/nibble/internal/buildchunk"
	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/embed"
	"github.com/bluesky585/nibble/pkg/store"
)

const maxBody = 10 << 20

// API holds process-local indexed chunks.
type API struct {
	mem *store.Memory
}

// New returns an API with an empty memory store.
func New() *API {
	return &API{mem: &store.Memory{}}
}

// Handler is the HTTP API. Each call gets its own memory index.
func Handler() http.Handler {
	return New().Handler()
}

// Handler serves health, chunk, and index routes.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /v1/chunk", chunkText)
	mux.HandleFunc("POST /v1/index", a.indexText)
	return mux
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type chunkRequest struct {
	Text      string `json:"text"`
	Chunker   string `json:"chunker"`
	Tokenizer string `json:"tokenizer"`
	Size      int    `json:"size"`
	Overlap   int    `json:"overlap"`
	Embedder  string `json:"embedder"`
}

type chunkResponse struct {
	Chunks []chunk.Chunk `json:"chunks"`
}

type indexResponse struct {
	Count int `json:"count"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func chunkText(w http.ResponseWriter, r *http.Request) {
	chunks, _, status, err := prepare(w, r)
	if err != nil {
		writeJSON(w, status, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, chunkResponse{Chunks: chunks})
}

func (a *API) indexText(w http.ResponseWriter, r *http.Request) {
	chunks, emb, status, err := prepare(w, r)
	if err != nil {
		writeJSON(w, status, errorResponse{Error: err.Error()})
		return
	}
	if err := store.Index(a.mem, emb, chunks); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, indexResponse{Count: len(chunks)})
}

func prepare(w http.ResponseWriter, r *http.Request) ([]chunk.Chunk, embed.Embedder, int, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)

	var req chunkRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return nil, nil, http.StatusBadRequest, err
	}

	if req.Chunker == "" {
		req.Chunker = "recursive"
	}
	if req.Tokenizer == "" {
		req.Tokenizer = "character"
	}
	if req.Size == 0 {
		req.Size = 512
	}

	emb, err := embed.Lookup(req.Embedder)
	if err != nil {
		return nil, nil, http.StatusBadRequest, err
	}

	c, err := buildchunk.New(req.Chunker, req.Tokenizer, req.Size, req.Overlap, emb)
	if err != nil {
		return nil, nil, http.StatusBadRequest, err
	}

	chunks, err := c.Chunk(req.Text)
	if err != nil {
		return nil, nil, http.StatusInternalServerError, err
	}
	if chunks == nil {
		chunks = []chunk.Chunk{}
	}
	return chunks, emb, 0, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
