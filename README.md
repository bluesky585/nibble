# nibble

A Go library for splitting long text into retrieval-sized chunks for RAG.

The name means a small bite: chunks should be small enough to retrieve on their own, and complete enough to keep context.

## Install

Requires Go 1.26+.

```bash
go get github.com/bluesky585/nibble@v0.2.0
go install github.com/bluesky585/nibble/cmd/nibble@v0.2.0
go install github.com/bluesky585/nibble/cmd/nibble-api@v0.2.0
```

## Principles

- **Reconstructable.** With no overlap, concatenating chunks in order must equal the original text.
- **Correct offsets.** Each chunk carries a half-open range `[Start, End)`. Offsets count Unicode code points (`rune` in Go), not bytes.
- **Deterministic.** The same input and config always produce the same chunks.
- **Separate measures.** Characters, bytes, and tokens are different rulers. Do not mix them.
- **Small core.** Chunking is the product. Embeddings and stores are swap-in interfaces, not a catalog of vendors.
- **One dependency, isolated.** The library is standard library only. A real token budget needs a BPE vocabulary, so `-tokenizer tiktoken` pulls in one third-party package; it lives in `pkg/tokenizer/tiktoken` and nothing else imports it, so `pkg/tokenizer` and every chunker still build with no dependency.
- **English only.** Code, comments, commit messages, and docs in this repo are written in English.

## v0.1 status

This is a first cut, not a production RAG platform. Chunking is the
finished part; retrieval is enough to try, not enough to deploy.

- The default tokenizer counts runes, so `-size` is not a model's token
  budget unless `-tokenizer tiktoken` is set.
- `-embedder hashing` is bag-of-words similarity, not a neural model.
  `-embedder openai` is a real model and needs network and a key.
- The only store is a JSONL file scanned linearly, so search is brute
  force over whatever the file holds.
- `POST /v1/index` keeps vectors in process memory; they are gone when
  the server exits.
- `code` parses Go only.
- A query must use the same embedder that built the index; a mismatch is
  reported rather than scored.

## CLI

```bash
go run ./cmd/nibble -chunker recursive -size 512 path/to/file.txt
```

Reads a UTF-8 file, a directory (`-dir`), or stdin. Prints a JSON array of chunks. `-dir` prints documents: `[{"path","content","chunks"}, ...]`.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-chunker` | `recursive` | `recursive`, `sentence`, `token`, `fast`, `table`, `code`, `markdown`, or `semantic` |
| `-tokenizer` | `character` | `character`, `word`, or `tiktoken` (ignored by `fast`) |
| `-size` | `512` | max tokens per chunk; **max bytes** for `fast` |
| `-overlap` | `0` | token overlap; token chunker only |
| `-index` | | optional JSONL file of chunk embeddings |
| `-embedder` | `hashing` | `hashing` or `openai` (used by `semantic` and `-index`) |
| `-context` | `0` | neighbor tokens copied into `context` (0 disables) |
| `-context-mode` | `prefix` | `prefix` or `suffix` |
| `-dir` | | recursive directory (not with a file argument) |
| `-ext` | `.txt,.md` | extensions for `-dir` |
| `-html` | | write an HTML page of the source colored by chunk |
| `-embed` | `false` | add an `embedding` to each chunk in the JSON output |
| `-query` | | search an `-index` file and print hits instead of chunking |
| `-k` | `5` | number of hits for `-query` |

`fast` looks for a delimiter near the byte budget and never splits a UTF-8 rune. JSON `start`/`end` are still rune offsets.

`tiktoken` counts with a real BPE vocabulary, so `-size` is a model's token budget: `-tokenizer tiktoken -size 512` means 512 cl100k_base tokens. The encoding table is not embedded. The first use downloads it once and caches it on disk (`TIKTOKEN_CACHE_DIR` overrides the location), so a run that never selects this tokenizer never touches the network. Pieces are cut on character boundaries rather than raw token boundaries, because a BPE token can end inside a character and offsets are rune ranges; a token ending mid-character yields an empty piece, so `Count` still equals `len(Split)` and joining the pieces still restores the input.

`table` splits GitHub-flavored Markdown tables by row. Later row-groups copy the header into `context` so retrieval keeps column names; `text` stays a slice of the original, so reconstruct still works.

`code` splits Go source on top-level declarations (package, types, funcs), keeping doc comments with the decl. If the file does not parse, it falls back to token windows.

`markdown` routes each region of a document to the chunker that fits it: fenced code blocks to `code`, GFM tables to `table`, everything else to `recursive`. Regions are disjoint and cover the whole input, so reconstruct still holds. A code block is split with its fence lines removed and reattached to the boundary chunks, since the fence syntax is not part of the language inside; this can push a boundary chunk slightly over `-size`.

`semantic` embeds sentences and starts a new chunk when cosine similarity drops below 0.5 or the token budget is full.

`sentence` and `semantic` cut a single sentence that is over budget on its own into token windows, so a long run without punctuation cannot push a chunk past `-size`. The only chunk that may still exceed `-size` is one holding a single token wider than the budget.

`-embedder hashing` is local and needs no network. `-embedder openai` calls an OpenAI-compatible `/v1/embeddings` API (`OPENAI_API_KEY`, optional `OPENAI_BASE_URL`, `OPENAI_EMBED_MODEL`). One HTTP request per batch.

`-index` writes chunks plus vectors from the selected embedder to JSONL.

`-embed` puts the vector on each chunk in the JSON on stdout, so you can look at embeddings without writing an index. Every input in the run is embedded together in bounded batches, so a directory does not cost one request per file nor one unbounded request. With `-index` the vectors are reused rather than computed twice, and are stored once (on the record, not also on the chunk). A chunk's vector is computed from `context + text` where `context` is set, matching what `-index` stores.

`-query` searches an existing index and prints the top `-k` hits as JSON, each with its score and the chunk it points at. It reads no input, so it never waits on stdin. Use the same `-embedder` that built the index: a query embedded by a different model has a different width, and that is reported as an error rather than scored as a meaningless ranking.

```bash
nibble -index docs.jsonl docs.txt
nibble -query "how do cats sleep" -index docs.jsonl -k 3
```

`-context` / `-context-mode` copy neighboring tokens into `context` without changing `text`, so reconstruct still holds. This is separate from `-overlap` (token windows).

`-html out.html` writes a self-contained page showing the source text colored by chunk, alongside a legend of offsets and token counts. It is a way to eyeball a split rather than count it. stdout is still the usual JSON. The page always reads exactly as the source, so it doubles as a check: overlapping chunks show their repeated part once and are flagged, text no chunk covers is hatched, and a chunk whose offsets disagree with the source is flagged rather than trusted.

## HTTP API

```bash
go run ./cmd/nibble-api -addr 127.0.0.1:8080
```

| Method | Path | Meaning |
| --- | --- | --- |
| `GET` | `/health` | liveness |
| `POST` | `/v1/chunk` | chunk JSON body |
| `POST` | `/v1/index` | chunk, embed, keep in process memory |
| `POST` | `/v1/search` | cosine search over that memory index |

`POST /v1/chunk` body:

```json
{
  "text": "Hello. World.",
  "chunker": "recursive",
  "tokenizer": "character",
  "size": 512,
  "overlap": 0,
  "embedder": "hashing"
}
```

Omitted fields use the same defaults as the CLI. `POST /v1/chunk` returns `{"chunks":[...]}`. `POST /v1/index` uses the same body and returns `{"count":N}`.

`POST /v1/search` body: `{"query":"cats","k":1,"embedder":"hashing"}`. `k` defaults to 5. Index and search must hit the same process.

## Development

Requires Go 1.26+.

```bash
go test ./...
go build -o nibble ./cmd/nibble
go build -o nibble-api ./cmd/nibble-api
```

## License

MIT. See [LICENSE](LICENSE).
