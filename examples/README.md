# Examples

Small, runnable programs, one per idea. Each is its own `main`
package, so each runs directly from a checkout:

```bash
go run ./examples/recursive
go run ./examples/store-search
go run ./examples/rag-search
```

`rag-search` is the retrieval lifecycle in one program: chunk with a
real chunker, index into SQLite under per-document sources, rank with
hybrid scoring, narrow a search to one source, and delete a source.

The examples favor short inputs and printed output you can check by
eye. Error handling is minimal on purpose — real programs should
handle errors, and the example comments point at where.
