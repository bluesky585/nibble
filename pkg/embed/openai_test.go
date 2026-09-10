package embed

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIEmbedBatch(t *testing.T) {
	t.Parallel()

	var gotCalls int
	var gotInput []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCalls++
		if r.URL.Path != "/v1/embeddings" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("auth=%s", r.Header.Get("Authorization"))
		}
		raw, _ := io.ReadAll(r.Body)
		var req openaiRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Errorf("decode: %v", err)
		}
		gotInput = req.Input
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(openaiResponse{
			Data: []struct {
				Index     int       `json:"index"`
				Embedding []float64 `json:"embedding"`
			}{
				{Index: 1, Embedding: []float64{0, 1}},
				{Index: 0, Embedding: []float64{1, 0}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	cli, err := NewOpenAI(srv.Client(), srv.URL, "sk-test", "test-model")
	if err != nil {
		t.Fatal(err)
	}

	got, err := cli.Embed([]string{"cats", "quantum"})
	if err != nil {
		t.Fatal(err)
	}
	if gotCalls != 1 {
		t.Fatalf("calls=%d want 1 (batch)", gotCalls)
	}
	if len(gotInput) != 2 || gotInput[0] != "cats" || gotInput[1] != "quantum" {
		t.Fatalf("input=%v", gotInput)
	}
	if len(got) != 2 || got[0][0] != 1 || got[1][1] != 1 {
		t.Fatalf("vectors not ordered by index: %v", got)
	}
}

func TestOpenAIHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad key"}}`))
	}))
	t.Cleanup(srv.Close)

	cli, err := NewOpenAI(srv.Client(), srv.URL, "sk-bad", "test-model")
	if err != nil {
		t.Fatal(err)
	}
	_, err = cli.Embed([]string{"hi"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOpenAIRequiresKey(t *testing.T) {
	t.Parallel()

	_, err := NewOpenAI(nil, "", "", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOpenAIEmptyBatch(t *testing.T) {
	t.Parallel()

	cli, err := NewOpenAI(http.DefaultClient, "http://127.0.0.1:1", "sk", "m")
	if err != nil {
		t.Fatal(err)
	}
	got, err := cli.Embed(nil)
	if err != nil || got != nil {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestLookup(t *testing.T) {
	t.Parallel()

	e, err := Lookup("hashing")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := e.(Hashing); !ok {
		t.Fatalf("got %T", e)
	}
	_, err = Lookup("nope")
	if err == nil {
		t.Fatal("expected unknown embedder")
	}
}
