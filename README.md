# nibble

A Go library for splitting long text into retrieval-sized chunks for RAG.

The name means a small bite: chunks should be small enough to retrieve on their own, and complete enough to keep context.

## Install

Requires Go 1.26+.

```bash
go get github.com/bluesky585/nibble@v0.6.0
go install github.com/bluesky585/nibble/cmd/nibble@v0.6.0
go install github.com/bluesky585/nibble/cmd/nibble-api@v0.6.0
```

## Quick start

### As a library

```go
package main

import (
	"fmt"

	"github.com/bluesky585/nibble/pkg/recursive"
	"github.com/bluesky585/nibble/pkg/tokenizer"
)

func main() {
	// Character counts one token per rune, so size is a rune budget. Use
	// tokenizer/tiktoken for a model's real token budget.
	c, err := recursive.New(tokenizer.Character{}, 30, nil)
	if err != nil {
		panic(err)
	}

	text := "Cats sleep twelve to sixteen hours a day. Dogs sleep ten to fourteen."
	chunks, err := c.Chunk(text)
	if err != nil {
		panic(err)
	}

	var joined string
	for _, ch := range chunks {
		fmt.Printf("[%d,%d) %d tokens %q\n", ch.Start, ch.End, ch.TokenCount, ch.Text)
		joined += ch.Text
	}
	fmt.Println("reconstructs:", joined == text)
}
```

```text
[0,29) 29 tokens "Cats sleep twelve to sixteen "
[29,41) 12 tokens "hours a day."
[41,69) 28 tokens " Dogs sleep ten to fourteen."
reconstructs: true
```

Three things to notice in that output, because they are the whole contract:

- `Start` and `End` are a half-open range of **rune** offsets, not byte
  offsets and not string indexes in other languages.
- Joining the chunks in order returns the input exactly. That is the
  first principle below, and it is checked by the tests on every commit.
- No cut landed mid-word. `recursive` walks down its rule stack (blank
  line, then line, then sentence, then word) and only falls to the next
  rule when the larger one does not fit, so the first cut here lands on a
  space. A cut inside a word is still possible — it is what happens when
  a single word is wider than the budget on its own — but it is the last
  resort, not the default.

Each chunker is its own package with the same one-method interface, so any
of them can stand in for `recursive` above:

| Package | Constructor | Cuts on |
| --- | --- | --- |
| `recursive` | `New(tok, size, rules)` | the largest of blank line / line / sentence / word that fits |
| `sentencechunker` | `New(tok, size, delims, opts...)` | sentence ends only |
| `tokenchunker` | `New(tok, size, overlap)` | fixed windows, optional repeat |
| `fastchunker` | `New(size, delims)` | a delimiter near a **byte** budget; no tokenizer |
| `tablechunker` | `New(tok, size)` | GFM table rows, with the header copied into `Context` |
| `codechunker` | `New(tok, size, opts...)` | top-level declarations; `codechunker.Language("go")` |
| `markdownchunker` | `New(tok, size)` | routes each region to the chunker that fits it |
| `semantic` | `New(tok, emb, size, minSim, opts...)` | cosine similarity drops between sentences |

`nil` rules and delims mean the package defaults. `tokenizer.Character{}`,
`tokenizer.Word{}`, and `tokenizer/tiktoken.New("cl100k_base")` are the three
tokenizers. The sentence-based chunkers take one extra option:
`sentencechunker.MinRunes(4)` merges sentence pieces shorter than 4 runes
into the next piece, which absorbs abbreviation fragments such as the `e.`
and `g.` that `e.g.` yields under a `.` delimiter. It is opt-in and off by
default, because the right value depends on the script: in CJK a two-rune
sentence is complete, and merging it with its neighbor destroys a real
boundary.

### From the command line

```bash
printf 'Cats sleep. Dogs bark. Birds sing.' | go run ./cmd/nibble -chunker sentence -size 12
```

No file needed: the CLI reads a file argument, a directory (`-dir`), or
stdin. `go run ./cmd/nibble` is for working inside a checkout; once
installed, the same command is just `nibble`. It prints:

```json
[
  {
    "text": "Cats sleep.",
    "start": 0,
    "end": 11,
    "token_count": 11
  },
  {
    "text": " Dogs bark.",
    "start": 11,
    "end": 22,
    "token_count": 11
  },
  {
    "text": " Birds sing.",
    "start": 22,
    "end": 34,
    "token_count": 12
  }
]
```

Add `-html out.html` to any run to get the same split colored over the
source text, which is usually faster to check by eye than by reading JSON.

## Principles

- **Reconstructable.** With no overlap, concatenating chunks in order must equal the original text.
- **Correct offsets.** Each chunk carries a half-open range `[Start, End)`. Offsets count Unicode code points (`rune` in Go), not bytes.
- **Deterministic.** The same input and config always produce the same chunks.
- **Separate measures.** Characters, bytes, and tokens are different rulers. Do not mix them.
- **Small core.** Chunking is the product. Embeddings and stores are swap-in interfaces, not a catalog of vendors.
- **One dependency, isolated.** The chunkers are standard library only. A real token budget needs a BPE vocabulary, so `-tokenizer tiktoken` pulls in one third-party package, and it is quarantined in `pkg/tokenizer/tiktoken`: every chunker, and `pkg/tokenizer` itself, still builds with no dependency. The programs import it, since a flag has to reach the tokenizer it names. HTML and EPUB extraction (`pkg/extract`) is quarantined the same way: it uses the official `golang.org/x/net` HTML parser, and every chunker still builds without it. The TypeScript client in `ts/` is a separate toolchain with its own dev dependencies (`typescript`, `@types/node`); it is a separate language for a separate consumer, and nothing in Go imports it.
- **English only.** Code, comments, commit messages, and docs in this repo are written in English.

## Status

This is a first cut, not a production RAG platform. Chunking is the
finished part; retrieval is enough to try, not enough to deploy.

- The default tokenizer counts runes, so `-size` is not a model's token
  budget unless `-tokenizer tiktoken` is set.
- `-embedder hashing` is bag-of-words similarity, not a neural model.
  `-embedder openai` is a real model and needs network and a key.
- The stores are a JSONL file and a SQLite database, both scanned
  linearly, so search is brute force over whatever the file holds. The
  store interface is where an approximate-nearest-neighbor backend
  would slot in if one ever earns its place.
- `POST /v1/index` keeps vectors in process memory; they are gone when
  the server exits.
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
| `-overlap` | `0` | token overlap; `token` widens windows, `recursive` repeats the previous chunk's tail |
| `-index` | | optional index file of chunk embeddings; `.db`/`.sqlite`/`.sqlite3` names a SQLite database, anything else JSONL |
| `-embedder` | `hashing` | `hashing` or `openai` (used by `semantic` and `-index`) |
| `-context` | `0` | neighbor tokens copied into `context` (0 disables) |
| `-context-mode` | `prefix` | `prefix` or `suffix` |
| `-dir` | | recursive directory (not with a file argument) |
| `-ext` | `.txt,.md` | extensions for `-dir` (`.csv`, `.tsv`, `.html`, `.epub` also work) |
| `-lang` | | language for `-chunker code`: `go` or `python` (empty detects it) |
| `-rules` | | JSON file with a rule hierarchy for `-chunker recursive` |
| `-html` | | write an HTML page of the source colored by chunk |
| `-embed` | `false` | add an `embedding` to each chunk in the JSON output |
| `-query` | | search an `-index` file and print hits instead of chunking |
| `-k` | `5` | number of hits for `-query` |
| `-scoring` | `dense` | `dense`, `bm25`, or `hybrid` (used by `-query`) |
| `-hybrid-weight` | `0.5` | dense share of the blend when `-scoring hybrid` (0 to 1) |

`fast` looks for a delimiter near the byte budget and never splits a UTF-8 rune. JSON `start`/`end` are still rune offsets.

`tiktoken` counts with a real BPE vocabulary, so `-size` is a model's token budget: `-tokenizer tiktoken -size 512` means 512 cl100k_base tokens. The encoding table is not embedded. The first use downloads it once and caches it on disk (`TIKTOKEN_CACHE_DIR` overrides the location), so a run that never selects this tokenizer never touches the network. Pieces are cut on character boundaries rather than raw token boundaries, because a BPE token can end inside a character and offsets are rune ranges; a token ending mid-character yields an empty piece, so `Count` still equals `len(Split)` and joining the pieces still restores the input.

`table` splits GitHub-flavored Markdown tables by row. Later row-groups copy the header into `context` so retrieval keeps column names; `text` stays a slice of the original, so reconstruct still works.

CSV and TSV files are read as tabular input wherever a file or `-dir` entry is accepted: one chunk per data row, with the column names carried in `context` the same way. Quoted fields, embedded delimiters, and embedded newlines are decoded by the CSV reader. The row is the unit of meaning — a row wider than `-size` is never cut, since splitting one destroys the column-to-value pairing. The header row is metadata, not content, so it never becomes a chunk; a file with no data rows yields none.

HTML (`.html`, `.htm`, `.xhtml`) and EPUB (`.epub`) files are extracted to plain text before chunking, wherever a file or `-dir` entry is accepted. Block-level tags become the paragraph breaks the recursive rules split on, inline tags disappear, script and style content is dropped, and entities decode to characters. An EPUB's spine, not the archive's file order, decides the sequence of its documents. What remains is ordinary text — offsets point into the extracted text, and reconstruct holds against it — so the chunkers never see the markup.

`code` splits source on top-level declarations, keeping a doc comment or
decorator with the declaration it documents. `-lang go` or `-lang python`
names the language; with no `-lang` it detects, and source it cannot cut
falls back to token windows. Go is read with the standard library parser,
so a declaration's span is exact. Python has no parser in the standard
library and nibble will not take a dependency for one, so it is read by a
line scanner that tracks bracket depth, comments, and string literals, and
cuts where a statement at column 0 begins. The scanner is deliberately
conservative: anything it cannot be sure about stays in the piece it is in,
because a missing boundary only makes the token fallback do more work,
while a wrong one would split a statement in half.

`markdown` routes each region of a document to the chunker that fits it: fenced code blocks to `code`, GFM tables to `table`, everything else to `recursive`. Regions are disjoint and cover the whole input, so reconstruct still holds. A fenced block's info string names its language, so each block is cut with the rules for what it holds (`python` blocks by Python rules, `go` blocks by Go rules) rather than being parsed as the wrong one; `-lang` is rejected here, since the document supplies it. A code block is split with its fence lines removed and reattached to the boundary chunks, since the fence syntax is not part of the language inside; this can push a boundary chunk slightly over `-size`.

`recursive` reads its rule hierarchy from defaults, from `[]recursive.Level` in code, or from a JSON file passed with `-rules` (`recursive.RulesFromJSON` parses the same shape in a request's `rules` field). One array element per level, coarsest first; `"token": true` hard-splits with the tokenizer, otherwise the level splits on its `"delimiters"`:

```json
[
  {"delimiters": ["\n\n"], "attach": "prev"},
  {"delimiters": ["。", ".", "!", "?"], "attach": "prev"},
  {"token": true}
]
```

`attach` is optional and only `"prev"` is accepted, because nibble always attaches a delimiter to the piece it ends. An empty array means the default hierarchy.

A chunk that would hold nothing but whitespace is never emitted: when a paragraph exactly fills the budget, its trailing blank-line separator would otherwise hard-split into a content-free chunk, which is pure retrieval noise. That separator is merged into the neighboring chunk instead — the previous one for a trailing blank, the next one for a run of blanks before the first content — so reconstruct still holds and the neighbor may exceed `-size` by the whitespace it absorbed.

`recursive` also accepts `-overlap`: the next chunk repeats the last `-overlap` tokens of the previous chunk's tail in its own `text`, the way the `token` chunker widens a window, so a retrieval hit on either side of a cut can see across it. Each chunk is still a slice of the source with exact offsets and still ends at the boundary the rules chose; only its start moves back. With a positive overlap the chunks are no longer a concatenation of the input — that is the same guarantee `token` overlap already trades away — and a chunk may exceed `-size` by up to the overlap, since the repeated run rides on top of content that already filled the budget. A previous chunk shorter than the overlap is repeated whole.

`semantic` embeds sentences and starts a new chunk when cosine similarity drops below 0.5 or the token budget is full. `semantic.SimilarityWindow(n)` widens that test from adjacent sentence pairs to the mean vectors of `n` sentences on each side of the cut point. A window of 2 or 3 smooths single-sentence wording jitter that would otherwise split an unchanged topic, at the cost of needing `n` sentences of context on both sides; points without it are not evaluated. Consecutive points that fall below the threshold are one boundary, cut at the deepest.

`sentence` and `semantic` cut a single sentence that is over budget on its own into token windows, so a long run without punctuation cannot push a chunk past `-size`. The only chunk that may still exceed `-size` is one holding a single token wider than the budget.

`-embedder hashing` is local and needs no network. `-embedder openai` calls an OpenAI-compatible `/v1/embeddings` API (`OPENAI_API_KEY`, optional `OPENAI_BASE_URL`, `OPENAI_EMBED_MODEL`). One HTTP request per batch.

`-index` writes chunks plus vectors from the selected embedder to an index file. The file's extension picks the store: `.db` (or `.sqlite`, `.sqlite3`) opens a SQLite database — one table, one row per chunk, keyed by text and offsets so a re-upserted chunk updates rather than duplicates, and vectors stored as float32 — while any other name writes JSONL. The two stores return the same ranking, so switching one for the other is a persistence choice, not a retrieval choice; the SQLite file is incremental (an upsert writes the rows it touched) and reopens across processes, where the JSONL store rewrites its file on every upsert.

```bash
nibble -index docs.db docs.txt
nibble -query "how do cats sleep" -index docs.db -k 3
```

`-embed` puts the vector on each chunk in the JSON on stdout, so you can look at embeddings without writing an index. Every input in the run is embedded together in bounded batches, so a directory does not cost one request per file nor one unbounded request. With `-index` the vectors are reused rather than computed twice, and are stored once (on the record, not also on the chunk). A chunk's vector is computed from `context + text` where `context` is set, matching what `-index` stores.

`-query` searches an existing index and prints the top `-k` hits as JSON, each with its score and the chunk it points at. It reads no input, so it never waits on stdin. Use the same `-embedder` that built the index: a query embedded by a different model has a different width, and that is reported as an error rather than scored as a meaningless ranking.

`-scoring` picks how hits are ranked. `dense` is cosine over the stored vectors, the default and the only mode that needs the embedder. `bm25` ranks by term overlap over the index texts and needs no embedder at all — it works on an index built without vectors, and it is the better ranking when the query is a rare word the vector model dilutes. `hybrid` blends both: `weight * dense + (1-weight) * bm25`, with the BM25 side normalized to the index's own maximum first, since BM25 scores are unbounded and would otherwise swamp the blend. Scores across the three modes are not comparable — each is its own measure.

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
| `POST` | `/v1/search` | search that memory index; `scoring` picks `dense` (default), `bm25`, or `hybrid` |

`POST /v1/chunk` body:

```json
{
  "text": "Hello. World.",
  "chunker": "recursive",
  "tokenizer": "character",
  "size": 512,
  "overlap": 0,
  "lang": "",
  "rules": "",
  "embedder": "hashing"
}
```

Omitted fields use the same defaults as the CLI. `rules` holds the same JSON rule hierarchy as the `-rules` file, inline, and only applies to the `recursive` chunker. `POST /v1/chunk` returns `{"chunks":[...]}`. `POST /v1/index` uses the same body and returns `{"count":N}`.

`POST /v1/search` body: `{"query":"cats","k":1,"embedder":"hashing"}`. `k` defaults to 5. Index and search must hit the same process. `scoring` picks the ranking, with the same meaning as the CLI's `-scoring`: `dense` (default) embeds the query and ranks by cosine, `bm25` ranks by term overlap and needs no embedder, and `hybrid` blends both with `hybrid_weight` as the dense share (default 0.5).

### Client

A thin TypeScript client for this API lives in [`ts/`](ts/). It calls a running
`nibble-api` and does not reimplement any chunking, so it cannot drift from the
server. Node 23.6.0+ runs it directly, with no build step and no npm publish.

```ts
import { NibbleClient } from "./ts/client.ts";

const client = new NibbleClient("http://127.0.0.1:8080");
const chunks = await client.chunk({ text, chunker: "recursive", size: 512 });
```

Note that a chunk's `start`/`end` are rune offsets, not string indexes, so
`text.slice(start, end)` is wrong for text holding an astral character. See
[`ts/README.md`](ts/README.md).

## Development

Requires Go 1.26+.

```bash
go test ./...
go build -o nibble ./cmd/nibble
go build -o nibble-api ./cmd/nibble-api
```

Benchmarks cover the core packages against a fixed corpus in
`internal/corpus` (a few kilobytes each, with CJK, a table, and Go and
Python sources mixed in). They are a manual baseline, not a CI gate —
compare before and after a change on the same machine:

```bash
go test -run xxx -bench=. -benchmem ./pkg/... 
```

The `ts/` client is a second toolchain and needs Node 23.6.0+:

```bash
cd ts && npm ci && npm run typecheck && npm test
```

CI runs both: `gofmt` and `go test ./...` for Go, and the type check and tests
for the client.

## License

MIT. See [LICENSE](LICENSE).
