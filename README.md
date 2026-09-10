# nibble

A Go library for splitting long text into retrieval-sized chunks for RAG.

The name means a small bite: chunks should be small enough to retrieve on their own, and complete enough to keep context.

## Principles

- **Reconstructable.** With no overlap, concatenating chunks in order must equal the original text.
- **Correct offsets.** Each chunk carries a half-open range `[Start, End)`. Offsets count Unicode code points (`rune` in Go), not bytes.
- **Deterministic.** The same input and config always produce the same chunks.
- **Separate measures.** Characters, bytes, and tokens are different rulers. Do not mix them.
- **Small core.** Get chunking right first. Document parsing, embeddings, and vector stores are out of scope for now.
- **English only.** Code, comments, commit messages, and docs in this repo are written in English.

## CLI

```bash
go run ./cmd/nibble -chunker recursive -size 512 path/to/file.txt
```

Reads a UTF-8 file, or stdin if no path is given. Prints a JSON array of chunks.

| Flag | Default | Meaning |
| --- | --- | --- |
| `-chunker` | `recursive` | `recursive`, `sentence`, `token`, `fast`, `table`, `code`, or `semantic` |
| `-tokenizer` | `character` | `character` or `word` (ignored by `fast`) |
| `-size` | `512` | max tokens per chunk; **max bytes** for `fast` |
| `-overlap` | `0` | token overlap; token chunker only |
| `-index` | | optional JSONL file of chunk embeddings |
| `-embedder` | `hashing` | `hashing` or `openai` (used by `semantic` and `-index`) |

`fast` looks for a delimiter near the byte budget and never splits a UTF-8 rune. JSON `start`/`end` are still rune offsets.

`table` splits GitHub-flavored Markdown tables by row. Later row-groups copy the header into `context` so retrieval keeps column names; `text` stays a slice of the original, so reconstruct still works.

`code` splits Go source on top-level declarations (package, types, funcs), keeping doc comments with the decl. If the file does not parse, it falls back to token windows.

`semantic` embeds sentences and starts a new chunk when cosine similarity drops below 0.5 or the token budget is full.

`-embedder hashing` is local and needs no network. `-embedder openai` calls an OpenAI-compatible `/v1/embeddings` API (`OPENAI_API_KEY`, optional `OPENAI_BASE_URL`, `OPENAI_EMBED_MODEL`). One HTTP request per batch.

`-index` writes chunks plus vectors from the selected embedder to JSONL.

## HTTP API

```bash
go run ./cmd/nibble-api -addr 127.0.0.1:8080
```

| Method | Path | Meaning |
| --- | --- | --- |
| `GET` | `/health` | liveness |
| `POST` | `/v1/chunk` | chunk JSON body |
| `POST` | `/v1/index` | chunk, embed, keep in process memory |

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

Omitted fields use the same defaults as the CLI. `POST /v1/chunk` returns `{"chunks":[...]}`. `POST /v1/index` uses the same body and returns `{"count":N}`. Search over that index is a later endpoint.

## Development

Requires Go 1.26+.

```bash
go test ./...
go build -o nibble ./cmd/nibble
go build -o nibble-api ./cmd/nibble-api
```
