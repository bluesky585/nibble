# Examples

Small, runnable programs, one per idea. Each is its own `main`
package, so each runs directly from a checkout:

```bash
go run ./examples/recursive
go run ./examples/store-search
```

The examples favor short inputs and printed output you can check by
eye. Error handling is minimal on purpose — real programs should
handle errors, and the example comments point at where.
