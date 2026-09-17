// Package httpapi serves chunking over HTTP.
package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/bluesky585/nibble/internal/buildchunk"
	"github.com/bluesky585/nibble/pkg/chunk"
	"github.com/bluesky585/nibble/pkg/embed"
	"github.com/bluesky585/nibble/pkg/store"
)

const maxBody = 10 << 20

// API holds indexed chunks behind one handler. The store is either
// process memory (the default, gone when the process exits) or a SQLite
// file (NewPersistent, resumed by the next process on the same path).
// A store is not safe for concurrent use, so every handler that touches
// it goes through the API's mutex; the net/http package runs handlers
// concurrently.
type API struct {
	mu  sync.Mutex
	mem *store.Memory
	sql *store.SQLite
}

// New returns an API with an empty memory store.
func New() *API {
	return &API{mem: &store.Memory{}}
}

// NewPersistent returns an API whose index lives in the SQLite file at
// path. The file is created when missing and kept when present, so a
// process started on the same path resumes from what the last one
// indexed. Close releases the file.
func NewPersistent(path string) (*API, error) {
	if path == "" {
		return nil, fmt.Errorf("index path is required")
	}
	sq, err := store.OpenSQLite(path)
	if err != nil {
		return nil, err
	}
	return &API{sql: sq}, nil
}

// Close releases a persistent API's file. A memory API closes to
// nothing.
func (a *API) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sql == nil {
		return nil
	}
	err := a.sql.Close()
	a.sql = nil
	return err
}

// Handler is the HTTP API. Each call gets its own memory index.
func Handler() http.Handler {
	return New().Handler()
}

// Handler serves health, chunk, index, and search routes.
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /v1/chunk", chunkText)
	mux.HandleFunc("POST /v1/index", a.indexText)
	mux.HandleFunc("POST /v1/search", a.searchText)
	return mux
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type chunkRequest struct {
	Text      string `json:"text"`
	Chunker   string `json:"chunker"`
	Tokenizer string `json:"tokenizer"`
	Lang      string `json:"lang"`
	Rules     string `json:"rules"`
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

type searchRequest struct {
	Query        string  `json:"query"`
	K            int     `json:"k"`
	Embedder     string  `json:"embedder"`
	Scoring      string  `json:"scoring"`
	HybridWeight float64 `json:"hybrid_weight"`
}

type searchResponse struct {
	Hits []store.Hit `json:"hits"`
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
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.index(emb, chunks); err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, indexResponse{Count: len(chunks)})
}

// index writes chunks to whichever store the API holds. The caller
// holds a.mu.
func (a *API) index(emb embed.Embedder, chunks []chunk.Chunk) error {
	if a.sql != nil {
		return store.Index(a.sql, emb, chunks)
	}
	return store.Index(a.mem, emb, chunks)
}

func (a *API) searchText(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)

	var req searchRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}
	if strings.TrimSpace(req.Query) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "query is required"})
		return
	}
	if req.K == 0 {
		req.K = 5
	}
	if req.Scoring == "" {
		req.Scoring = store.RankDense
	}

	// The scoring mode decides what a query is: dense and hybrid embed
	// it, bm25 reads it as terms and needs no embedder at all.
	var queryVec []float64
	if req.Scoring != store.RankBM25 {
		emb, err := embed.Lookup(req.Embedder)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		vecs, err := emb.Embed([]string{req.Query})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
			return
		}
		queryVec = vecs[0]
	}

	// The whole search holds the lock: a store is not safe for
	// concurrent use, and an index arriving mid-search would otherwise
	// change the corpus under the scores.
	a.mu.Lock()
	records, err := a.records()
	if err == nil {
		var scores []float64
		scores, err = store.RankScores(records, req.Query, queryVec, req.Scoring, req.HybridWeight)
		if err == nil {
			hits := store.Rank(records, scores, req.K)
			if hits == nil {
				hits = []store.Hit{}
			}
			writeJSON(w, http.StatusOK, searchResponse{Hits: hits})
		}
	}
	a.mu.Unlock()
	if err != nil {
		// A file read failure is a server fault; a bad scoring request
		// is the caller's. RankScores produces the latter, records() the
		// former, and both end here.
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
	}
}

// records returns the API's whole corpus. The caller holds a.mu.
func (a *API) records() ([]store.Record, error) {
	if a.sql != nil {
		return a.sql.Records()
	}
	return a.mem.Records(), nil
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

	c, err := buildchunk.New(req.Chunker, req.Tokenizer, req.Lang, req.Rules, req.Size, req.Overlap, emb)
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
