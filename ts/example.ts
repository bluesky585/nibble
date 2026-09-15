/**
 * Walks the whole API against a running server.
 *
 *     go run ./cmd/nibble-api -addr 127.0.0.1:8080
 *     node ts/example.ts
 *     node ts/example.ts http://127.0.0.1:9000
 *
 * No npm install and no build step: Node strips the types and runs this file.
 */

import { NibbleClient, NibbleError } from "./client.ts";

const base = process.argv[2] ?? "http://127.0.0.1:8080";
const client = new NibbleClient(base);

try {
  await run();
} catch (err) {
  // A failure here is a missing or misconfigured server, not a bug in this
  // file, so it gets one line rather than a stack trace.
  console.error(err instanceof NibbleError ? `${err.message} (status ${err.status})` : err);
  process.exit(1);
}

async function run(): Promise<void> {
  if (!(await client.health())) {
    console.error(`no nibble-api is answering at ${base}`);
    process.exit(1);
  }

  await split();
  await offsets();
  await search();
}

/** Split by sentence, with a budget small enough to force a split. */
async function split(): Promise<void> {
  const text = "Cats sleep. Dogs bark. Birds sing.";
  const chunks = await client.chunk({ text, chunker: "sentence", size: 8 });
  for (const c of chunks) {
    console.log(`[${c.start},${c.end}) ${c.token_count} tokens ${JSON.stringify(c.text)}`);
  }
  console.log("reconstructs:", chunks.map((c) => c.text).join("") === text);
}

/**
 * A chunk's offsets count runes, and JavaScript slices by UTF-16 code unit,
 * so slicing by index is wrong for text holding an astral character. The
 * budget below is picked so a boundary lands on either side of one.
 *
 * This is the one thing to get right when showing a chunk to a user, and it
 * is easy to miss: the wrong slice is usually the right length minus a bit,
 * not an error.
 */
async function offsets(): Promise<void> {
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
    // The two agree until a chunk touches the astral character; from there on
    // the index slice is off by one, and it stays off by one.
    const mark = byIndex === c.text ? "both agree" : "index slice is WRONG";
    console.log(
      `[${c.start},${c.end}) text=${JSON.stringify(c.text)} rune=${JSON.stringify(byRunes)} index=${JSON.stringify(byIndex)} ${mark}`,
    );
    if (byRunes !== c.text) {
      throw new Error(`rune slice disagrees with the chunk at [${c.start},${c.end})`);
    }
  }
}

/**
 * Indexing keeps the vectors in the server's memory, so this has to reach the
 * same process that the split above did. The index appends, so running this
 * twice against one server searches both runs; restart the server for a clean
 * one.
 */
async function search(): Promise<void> {
  const text = "Cats sleep. Dogs bark. Birds sing.";
  const count = await client.index({ text, embedder: "hashing" });
  console.log(`indexed ${count} chunks`);

  const hits = await client.search({ query: "dogs bark", k: 2 });
  for (const h of hits) {
    console.log(`${h.score.toFixed(3)} ${JSON.stringify(h.record.chunk.text)}`);
  }
}
