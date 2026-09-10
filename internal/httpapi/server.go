// Package httpapi serves chunking over HTTP.
package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/bluesky585/nibble/internal/buildchunk"
	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/embed"
)

const maxBody = 10 << 20

// Handler is the HTTP API.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /v1/chunk", chunkText)
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

type errorResponse struct {
	Error string `json:"error"`
}

func chunkText(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)

	var req chunkRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
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
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	c, err := buildchunk.New(req.Chunker, req.Tokenizer, req.Size, req.Overlap, emb)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	chunks, err := c.Chunk(req.Text)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	if chunks == nil {
		chunks = []chunk.Chunk{}
	}
	writeJSON(w, http.StatusOK, chunkResponse{Chunks: chunks})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
