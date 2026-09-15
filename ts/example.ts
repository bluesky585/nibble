/**
 * Walks the whole API against a running server.
 *
 *     go run ./cmd/nibble-api -addr 127.0.0.1:8080
 *     node ts/example.ts
 *     node ts/example.ts http://127.0.0.1:9000
 *
 * No npm install and no build step: Node strips the types and runs this file.
 */

import { NibbleClient } from "./client.ts";

const base = process.argv[2] ?? "http://127.0.0.1:8080";
const client = new NibbleClient(base);

if (!(await client.health())) {
  console.error(`no nibble-api is answering at ${base}`);
  process.exit(1);
}

const text = "Cats sleep. Dogs bark. Birds sing.";

// Split by sentence, with a budget small enough to force a split.
const chunks = await client.chunk({ text, chunker: "sentence", size: 8 });
for (const c of chunks) {
  console.log(`[${c.start},${c.end}) ${c.token_count} tokens ${JSON.stringify(c.text)}`);
}
console.log("reconstructs:", chunks.map((c) => c.text).join("") === text);

// A chunk's offsets count runes, and JavaScript slices by UTF-16 code unit,
// so slicing by index is wrong for text holding an astral character. The
// budget below is picked so a boundary lands on either side of one.
//
// This is the one thing to get right when showing a chunk to a user, and it
// is easy to miss: the wrong slice is usually the right length minus a bit,
// not an error.
const astral = "abcdef🙂ghij";
const parts = await client.chunk({
  text: astral,
  chunker: "token",
  tokenizer: "character",
  size: 5,
});
for (const c of parts) {
  const byRunes = Array.from(astral).slice(c.start, c.end).join("");
  const byIndex = astral.slice(c.start, c.end);
  const verdict = byRunes === c.text ? "ok" : "MISMATCH";
  console.log(
    `[${c.start},${c.end}) runes=${JSON.stringify(byRunes)} index=${JSON.stringify(byIndex)} ${verdict}`,
  );
}

// Indexing keeps the vectors in the server's memory, so the search below has
// to reach the same process.
const count = await client.index({ text, embedder: "hashing" });
console.log(`indexed ${count} chunks`);

const hits = await client.search({ query: "dogs bark", k: 2 });
for (const h of hits) {
  console.log(`${h.score.toFixed(3)} ${JSON.stringify(h.record.chunk.text)}`);
}
