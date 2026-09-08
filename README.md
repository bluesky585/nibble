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
| `-chunker` | `recursive` | `recursive`, `sentence`, or `token` |
| `-tokenizer` | `character` | `character` or `word` |
| `-size` | `512` | max tokens per chunk |
| `-overlap` | `0` | token overlap; token chunker only |

## Development

Requires Go 1.26+.

```bash
go test ./...
go build -o nibble ./cmd/nibble
```
