import { test } from "node:test";
import assert from "node:assert/strict";

import { NibbleClient, NibbleError } from "./client.ts";
import type { Source } from "./client.ts";
import type {
  Chunk,
  Chunker,
  ChunkRequest,
  Embedder,
  FetchLike,
  Hit,
  Language,
  Scoring,
  SearchRequest,
  Tokenizer,
} from "./client.ts";

/** One recorded call, so a test can assert what actually went on the wire. */
interface Call {
  url: string;
  method: string;
  headers: Record<string, string>;
  body: string | undefined;
}

/**
 * A stand-in for fetch that answers from a queue and records the calls. A
 * real server is not started: these tests pin the wire format, and the API's
 * own Go tests already cover what happens on the other side of it.
 */
function recorder(replies: Array<Response | Error>): { fetch: FetchLike; calls: Call[] } {
  const calls: Call[] = [];
  const fetch: FetchLike = async (url, init) => {
    calls.push({
      url,
      method: init?.method ?? "GET",
      headers: init?.headers ?? {},
      body: init?.body,
    });
    const next = replies.shift();
    if (next === undefined) {
      throw new Error("recorder ran out of replies");
    }
    if (next instanceof Error) {
      throw next;
    }
    return next;
  };
  return { fetch, calls };
}

function json(status: number, value: unknown): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "content-type": "application/json" },
  });
}

/**
 * The call a test just made. The index is known good in these tests, so a
 * missing one is a bug in the test, and an empty object makes that read as a
 * failed field assertion rather than a stack trace here.
 */
function call(calls: Call[], i = 0): Call {
  return (
    calls[i] ?? { url: "", method: "", headers: {}, body: undefined }
  );
}

test("health reports the server state", async () => {
  const { fetch, calls } = recorder([json(200, { ok: true })]);
  const client = new NibbleClient("http://nibble.test", { fetch });

  assert.equal(await client.health(), true);
  assert.equal(call(calls).method, "GET");
  assert.equal(call(calls).url, "http://nibble.test/health");
  // A GET carries no body, so it must not announce a content-type.
  assert.equal(call(calls).body, undefined);
  assert.equal(call(calls).headers["content-type"], undefined);
});

test("chunk posts the request and returns the chunks", async () => {
  const chunks: Chunk[] = [
    { text: "Hello. ", start: 0, end: 7, token_count: 7 },
    { text: "World.", start: 7, end: 13, token_count: 6, context: "h" },
  ];
  const { fetch, calls } = recorder([json(200, { chunks })]);

  const client = new NibbleClient("http://nibble.test/", { fetch });
  const got = await client.chunk({ text: "Hello. World.", chunker: "sentence", size: 64 });

  assert.deepEqual(got, chunks);
  assert.equal(call(calls).method, "POST");
  // The trailing slash on the base URL is normalized, not doubled.
  assert.equal(call(calls).url, "http://nibble.test/v1/chunk");
  assert.equal(call(calls).headers["content-type"], "application/json");
  assert.deepEqual(JSON.parse(call(calls).body ?? ""), {
    text: "Hello. World.",
    chunker: "sentence",
    size: 64,
  });
});

// An option that was not passed must not be sent as null or "", because the
// API rejects an unknown value and a default only applies to an absent field.
test("chunk omits every option that was not given", async () => {
  const { fetch, calls } = recorder([json(200, { chunks: [] })]);

  const client = new NibbleClient("http://nibble.test", { fetch });
  assert.deepEqual(await client.chunk({ text: "hi" }), []);
  assert.deepEqual(JSON.parse(call(calls).body ?? ""), { text: "hi" });
});

test("index returns the stored count", async () => {
  const { fetch, calls } = recorder([json(200, { count: 3 })]);

  const client = new NibbleClient("http://nibble.test", { fetch });
  const count = await client.index({ text: "a b c", embedder: "hashing" });

  assert.equal(count, 3);
  assert.equal(call(calls).url, "http://nibble.test/v1/index");
  assert.deepEqual(JSON.parse(call(calls).body ?? ""), { text: "a b c", embedder: "hashing" });
});

test("search returns the hits", async () => {
  const hits: Hit[] = [
    {
      record: {
        chunk: { text: "cats sleep", start: 0, end: 10, token_count: 10 },
        vector: [0.5, 0.5],
      },
      score: 0.9,
    },
  ];
  const { fetch, calls } = recorder([json(200, { hits })]);

  const client = new NibbleClient("http://nibble.test", { fetch });
  const got = await client.search({ query: "cats", k: 3 });

  assert.deepEqual(got, hits);
  assert.equal(call(calls).url, "http://nibble.test/v1/search");
  assert.deepEqual(JSON.parse(call(calls).body ?? ""), { query: "cats", k: 3 });
});

// The server reports a bad option as {"error": "..."} with a 4xx. That
// message is the useful part, so it becomes the error message.
test("an error response becomes a NibbleError carrying the server message", async () => {
  const { fetch } = recorder([json(400, { error: 'unknown chunker "magic"' })]);

  const client = new NibbleClient("http://nibble.test", { fetch });
  await assert.rejects(
    () => client.chunk({ text: "hi", chunker: "magic" as Chunker }),
    (err: unknown) => {
      assert.ok(err instanceof NibbleError);
      assert.equal(err.status, 400);
      assert.equal(err.message, 'unknown chunker "magic"');
      return true;
    },
  );
});

test("a status without the API's error shape still reports that status", async () => {
  const { fetch } = recorder([
    new Response("<html>gateway</html>", { status: 502, headers: { "content-type": "text/html" } }),
  ]);

  const client = new NibbleClient("http://nibble.test", { fetch });
  await assert.rejects(
    () => client.health(),
    (err: unknown) => {
      assert.ok(err instanceof NibbleError);
      assert.equal(err.status, 502);
      assert.match(err.message, /502/);
      assert.match(err.body, /gateway/);
      return true;
    },
  );
});

// A refused connection never produced a status. Reporting 0 says "the request
// did not reach an API", which is different from a 500.
test("a transport failure is reported as status 0", async () => {
  const { fetch } = recorder([new TypeError("fetch failed")]);

  const client = new NibbleClient("http://nibble.test", { fetch });
  await assert.rejects(
    () => client.health(),
    (err: unknown) => {
      assert.ok(err instanceof NibbleError);
      assert.equal(err.status, 0);
      assert.match(err.message, /fetch failed/);
      return true;
    },
  );
});

test("a 200 that is not JSON is an error, not a silent empty result", async () => {
  const { fetch } = recorder([new Response("not json", { status: 200 })]);

  const client = new NibbleClient("http://nibble.test", { fetch });
  await assert.rejects(
    () => client.chunk({ text: "hi" }),
    (err: unknown) => {
      assert.ok(err instanceof NibbleError);
      assert.match(err.message, /not JSON/);
      return true;
    },
  );
});

test("extra headers are sent alongside the content-type", async () => {
  const { fetch, calls } = recorder([json(200, { chunks: [] })]);

  const client = new NibbleClient("http://nibble.test", {
    fetch,
    headers: { authorization: "Bearer token" },
  });
  await client.chunk({ text: "hi" });

  assert.equal(call(calls).headers["authorization"], "Bearer token");
  assert.equal(call(calls).headers["content-type"], "application/json");
});

// A fetch that cannot be called is fatal to every request, so it is reported
// once at construction rather than on the first call.
test("a fetch that is not callable is refused at construction", () => {
  assert.throws(
    () =>
      new NibbleClient("http://nibble.test", { fetch: 42 as unknown as FetchLike }),
    (err: unknown) => {
      assert.ok(err instanceof NibbleError);
      assert.equal(err.status, 0);
      assert.match(err.message, /not callable/);
      return true;
    },
  );
});

// Passing no fetch is not an error when the runtime has one: the client falls
// back to the global, which is the ordinary case.
test("no fetch option falls back to the global fetch", () => {
  const client = new NibbleClient("http://nibble.test");
  assert.equal(client.baseUrl, "http://nibble.test");
});

// The offsets are rune offsets. JS slicing is by UTF-16 code unit, and an
// astral character is two of them, so slicing by index cuts it in half. This
// is the mistake the client's docs point at, asserted so the docs stay true.
test("a rune slice is required for text holding astral characters", () => {
  const text = "a🙂b";
  const chunk: Chunk = { text: "🙂", start: 1, end: 2, token_count: 1 };

  assert.notEqual(text.slice(chunk.start, chunk.end), chunk.text);
  assert.equal(Array.from(text).slice(chunk.start, chunk.end).join(""), chunk.text);
  // The same check for a chunk that starts at 0 hides the bug, since only the
  // end is off: "a🙂" sliced by index is "a" plus half a surrogate pair.
  assert.equal(Array.from(text).slice(0, 2).join(""), "a🙂");
});

// The unions are the point of this client: a value the API would reject
// should be a compile error. tsc --noEmit fails the build if these stop
// holding, which is the only reason the annotations exist.
test("the option unions reject values the server does not accept", () => {
  const chunkers: Chunker[] = [
    "recursive",
    "sentence",
    "token",
    "fast",
    "table",
    "code",
    "markdown",
    "semantic",
  ];
  const tokenizers: Tokenizer[] = ["character", "word", "tiktoken"];
  const embedders: Embedder[] = ["hashing", "openai"];
  const scorings: Scoring[] = ["dense", "bm25", "hybrid"];
  const languages: Language[] = ["go", "golang", "python", "python3", "python2", "py", ""];

  // @ts-expect-error "magic" is not a chunker the server knows.
  const badChunker: Chunker = "magic";
  // @ts-expect-error "emoji" is not a tokenizer the server knows.
  const badTokenizer: Tokenizer = "emoji";
  // @ts-expect-error "rust" is not a language the server can cut.
  const badLanguage: Language = "rust";
  // @ts-expect-error lang is a string union, not an arbitrary string.
  const badLang: Language = "javascript";

  const req: ChunkRequest = { text: "hi", chunker: chunkers[0], tokenizer: tokenizers[0] };
  const search: SearchRequest = { query: "hi", embedder: embedders[0] };

  assert.equal(req.text, "hi");
  assert.equal(search.query, "hi");
  assert.equal(languages.length, 7);
  assert.equal(scorings.length, 3);
  // @ts-expect-error "splade" is not a scoring mode the server knows.
  const badScoring: Scoring = "splade";
  assert.equal(badScoring, "splade");
  // Read so the unused-variable check does not flag the type assertions.
  assert.equal(badChunker, "magic");
  assert.equal(badTokenizer, "emoji");
  assert.equal(badLanguage, "rust");
  assert.equal(badLang, "javascript");
});

test("listSources gets the index's origins", async () => {
  const sources: Source[] = [
    { name: "a.md", count: 2 },
    { name: "b.md", count: 1 },
  ];
  const { fetch, calls } = recorder([json(200, { sources })]);

  const client = new NibbleClient("http://nibble.test", { fetch });
  const got = await client.listSources();

  assert.deepEqual(got, sources);
  assert.equal(call(calls).method, "GET");
  assert.equal(call(calls).url, "http://nibble.test/v1/sources");
  assert.equal(call(calls).body, undefined);
});

test("deleteSource removes one origin and reports the count", async () => {
  const { fetch, calls } = recorder([json(200, { deleted: 2 })]);

  const client = new NibbleClient("http://nibble.test", { fetch });
  const deleted = await client.deleteSource("docs/a.md");

  assert.equal(deleted, 2);
  assert.equal(call(calls).method, "DELETE");
  // The name is a path parameter, so a slash in it must survive.
  assert.equal(call(calls).url, "http://nibble.test/v1/sources/docs%2Fa.md");
});

test("index sends the source when given", async () => {
  const { fetch, calls } = recorder([json(200, { count: 1 })]);

  const client = new NibbleClient("http://nibble.test", { fetch });
  await client.index({ text: "a", source: "a.md", embedder: "hashing" });

  assert.deepEqual(JSON.parse(call(calls).body ?? ""), {
    text: "a",
    source: "a.md",
    embedder: "hashing",
  });
});
