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

## Status

The repo is newly initialized. There is no chunker yet. Next: types, tokenizer, delimiter splitting, then chunkers.

## Development

Requires Go 1.26+.

```bash
go test ./...
```

There are no Go packages yet, so this command reports that no packages matched. That is expected.
