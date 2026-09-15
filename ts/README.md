# nibble TypeScript client

A thin client for the nibble HTTP API. It sends text to a running `nibble-api`
and reads back the chunks that server produced.

It is thin on purpose. Every chunker is Go, and they stay Go: nothing here
reimplements splitting, so the client and the server can never disagree about
what a chunk is. If you want a different chunker, add it to the Go library and
the client gets it for free.

## Requirements

Node 23.6.0 or newer. That is the version where Node started running `.ts`
files by stripping the types with no flag, which is why there is no build step
and no `dist/`. At 23.6.0 Node still prints an experimental warning for it;
that warning stops at 24.3.0 and the feature is stable from 24.12.0. CI pins
23.6.0, so the floor this file claims is the floor that is tested.

The TypeScript here sticks to erasable syntax — no `enum`, no `namespace`, no
constructor parameter properties — because those need code generation, and
Node only deletes syntax it can erase.

Node does not type check. It only deletes the types, so a wrong type is
invisible until something calls it. `npm run typecheck` runs `tsc` for that,
and CI runs it too.

## Install

Not published to npm, and it cannot be: Node refuses to run TypeScript under a
`node_modules` path, so an installed copy of `client.ts` would not run at all.
Use it from this repository, and compile it to JavaScript yourself if you need
to ship it.

```bash
git clone https://github.com/bluesky585/nibble
cd nibble/ts && npm ci
```

## Start the server

```bash
go run ./cmd/nibble-api -addr 127.0.0.1:8080
```

## Use

```ts
import { NibbleClient } from "./ts/client.ts";

const client = new NibbleClient("http://127.0.0.1:8080");

const chunks = await client.chunk({ text, chunker: "recursive", size: 512 });
const count = await client.index({ text, embedder: "hashing" });
const hits = await client.search({ query: "how do cats sleep", k: 3 });
```

`node ts/example.ts` runs all of that and prints the result.

## Rune offsets, not string indexes

This is the one thing that will bite you.

`start` and `end` are half-open **rune** offsets — Unicode code points, which
is what Go counts. JavaScript strings are indexed by UTF-16 code unit, and an
astral character (`🙂`, `𠀋`, most emoji) is two of them. So this is wrong:

```ts
text.slice(chunk.start, chunk.end); // wrong once text holds an astral character
```

and this is right:

```ts
Array.from(text).slice(chunk.start, chunk.end).join("");
```

It fails only for the chunks that touch a wide character, so a test document
of plain ASCII will not show you the bug.

## API

| Method | Path | Client |
| --- | --- | --- |
| `GET` | `/health` | `health(): Promise<boolean>` |
| `POST` | `/v1/chunk` | `chunk(request): Promise<Chunk[]>` |
| `POST` | `/v1/index` | `index(request): Promise<number>` |
| `POST` | `/v1/search` | `search(request): Promise<Hit[]>` |

Every option is optional, and an omitted option is left out of the request
rather than sent as `null`, so the server applies the same default it applies
to the CLI.

`index` stores vectors in the server's process memory. A `search` must reach
the same process that indexed, and must use the same `embedder`.

Failures throw `NibbleError`, whose `message` is the server's own message when
the server sent one, `status` is the HTTP status, and `body` is the raw
response. A transport failure that never reached an API has `status` 0.

## Test

```bash
cd ts
npm ci
npm run typecheck
npm test
```

`npm test` runs `node --test`. No server is needed: the tests stand in for
`fetch` and assert what goes on the wire, one call at a time. What happens on
the other side of that wire is covered by the Go tests in `internal/httpapi`.
