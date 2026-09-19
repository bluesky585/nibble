/**
 * A client for the nibble HTTP API.
 *
 * nibble splits long text into retrieval-sized chunks. The chunkers are Go
 * and they stay Go: this client does not reimplement any of them. It sends
 * text to a running `nibble-api` and reads back the chunks that server
 * produced, so the two can never disagree about what a chunk is.
 *
 * There is no build step and no npm package. Node runs this file directly by
 * stripping the types, which is why it sticks to erasable syntax (no enum, no
 * namespace, no constructor parameter properties).
 *
 * Start a server with:
 *
 *     go run ./cmd/nibble-api -addr 127.0.0.1:8080
 */

/**
 * A contiguous slice of the original text.
 *
 * `start` and `end` are half-open rune offsets: the chunk covers
 * `[start, end)` counted in Unicode code points, which is what Go counts.
 *
 * This is not what JavaScript counts. `"🙂"` is one rune but two UTF-16 code
 * units, so `text.slice(chunk.start, chunk.end)` is wrong as soon as the
 * document holds an astral character. Slice by runes:
 * `Array.from(text).slice(chunk.start, chunk.end).join("")`.
 */
export interface Chunk {
  text: string;
  /** Half-open rune offsets into the original text. */
  start: number;
  end: number;
  /** The chunker's own tally, not a recount of `text`. */
  token_count: number;
  /** Extra text for retrieval (a table header, a neighbour). Not part of `text`. */
  context?: string;
  /** Present only when the request asked for one. */
  embedding?: number[];
}

/**
 * A chunk plus the vector it was indexed with, as it appears in a hit.
 *
 * This is not named `Record`: that would shadow TypeScript's own
 * `Record<K, V>` utility type inside this module and in any file that
 * imports it, and `Record<string, string>` is too common to take away.
 */
export interface IndexedChunk {
  chunk: Chunk;
  vector: number[];
  /** Where this chunk came from, when indexed with a source. */
  source?: string;
}

/** One origin an index holds chunks from, with how many carry it. */
export interface Source {
  name: string;
  count: number;
}

/** One search result. */
export interface Hit {
  record: IndexedChunk;
  score: number;
}

/**
 * The chunker to split with. `fast` takes a byte budget instead of a token
 * budget, so its `size` is bytes while its offsets are still runes.
 */
export type Chunker =
  | "recursive"
  | "sentence"
  | "token"
  | "fast"
  | "table"
  | "code"
  | "markdown"
  | "semantic";

/** The ruler `size` is measured in. `fast` ignores this. */
export type Tokenizer = "character" | "word" | "tiktoken";

/** The embedder used by `semantic` and by index/search. */
export type Embedder = "hashing" | "openai";

/**
 * How hits are ranked. `dense` is cosine over stored vectors and is the
 * only mode that uses the embedder; `bm25` ranks by term overlap over
 * the indexed texts and needs no embedder at all; `hybrid` blends both.
 * Scores across modes are not comparable — each is its own measure.
 */
export type Scoring = "dense" | "bm25" | "hybrid";

/**
 * The language `chunker: "code"` should read. Omit it, or pass `""`, to let
 * the server detect it. `markdown` rejects this: each fence names its own
 * language.
 */
export type Language =
  | "go"
  | "golang"
  | "python"
  | "python3"
  | "python2"
  | "py"
  | "";

/** The body of `POST /v1/chunk` and `POST /v1/index`. */
export interface ChunkRequest {
  text: string;
  chunker?: Chunker;
  tokenizer?: Tokenizer;
  lang?: Language;
  /** Max tokens per chunk, or max bytes when `chunker` is `fast`. */
  size?: number;
  /**
   * Token overlap. `token` widens its windows; `recursive` repeats the
   * previous chunk's tail in the next chunk's text. Other chunkers
   * reject it.
   */
  overlap?: number;
  embedder?: Embedder;
  /**
   * Labels where these chunks came from, so `DELETE /v1/sources/{name}`
   * can replace them later. Optional; an upserted chunk under a new
   * source moves there.
   */
  source?: string;
}

/** The body of `POST /v1/search`. */
export interface SearchRequest {
  query: string;
  /** Number of hits to return. The server defaults to 5. */
  k?: number;
  embedder?: Embedder;
  /** How hits are ranked. The server defaults to `dense`. */
  scoring?: Scoring;
  /**
   * The dense share of a `hybrid` blend, from 0 (all bm25) to 1 (all
   * dense). The server defaults to 0.5. Ignored by the other modes.
   */
  hybrid_weight?: number;
}

/** Options for the client itself, not for a request. */
export interface ClientOptions {
  /**
   * The `fetch` to call. Pass one in to test without a server, or to use a
   * runtime whose `fetch` is not global.
   */
  fetch?: FetchLike;
  /** Headers sent with every request. */
  headers?: Record<string, string>;
  /** Milliseconds before a request is aborted. 0 disables the timeout. */
  timeoutMs?: number;
}

/** The shape of `fetch` this client needs. */
export type FetchLike = (
  input: string,
  init?: {
    method?: string;
    headers?: Record<string, string>;
    body?: string;
    signal?: AbortSignal;
  },
) => Promise<Response>;

/** An error response from the API, or a transport failure. */
export class NibbleError extends Error {
  /** The HTTP status, or 0 when the request never got that far. */
  readonly status: number;
  /** The response body as text, for a status the client did not expect. */
  readonly body: string;

  constructor(status: number, message: string, body = "") {
    super(message);
    this.name = "NibbleError";
    this.status = status;
    this.body = body;
  }
}

const defaultBaseUrl = "http://127.0.0.1:8080";

/**
 * Talks to one `nibble-api`.
 *
 * A client holds no connection of its own, so one instance is enough for a
 * whole run: `fetch` opens and pools what it needs underneath. Build one per
 * base URL rather than one per call.
 */
export class NibbleClient {
  readonly baseUrl: string;
  private readonly fetchImpl: FetchLike;
  private readonly headers: Record<string, string>;
  private readonly timeoutMs: number;

  constructor(baseUrl: string = defaultBaseUrl, options: ClientOptions = {}) {
    this.baseUrl = baseUrl.replace(/\/+$/, "");
    this.fetchImpl = options.fetch ?? (globalThis.fetch as FetchLike);
    this.headers = options.headers ?? {};
    this.timeoutMs = options.timeoutMs ?? 0;
    if (typeof this.fetchImpl !== "function") {
      // Either nothing was passed and this runtime has no global fetch, or
      // what was passed is not callable. Both are fatal to every request, so
      // they are reported once here rather than on each call.
      throw new NibbleError(0, "fetch is not callable: pass one in options.fetch");
    }
  }

  /** `GET /health`: whether the server is up. */
  async health(): Promise<boolean> {
    const body = await this.send<{ ok: boolean }>("GET", "/health");
    return body.ok === true;
  }

  /**
   * `POST /v1/chunk`: split text and return the chunks.
   *
   * The chunks are contiguous and cover the input, so joining their `text` in
   * order reproduces it, unless `overlap` was set.
   */
  async chunk(request: ChunkRequest): Promise<Chunk[]> {
    const body = await this.send<{ chunks: Chunk[] }>("POST", "/v1/chunk", request);
    return body.chunks;
  }

  /**
   * `POST /v1/index`: chunk, embed, and keep in the server's memory index.
   * Returns how many chunks were stored.
   *
   * The index lives in the server process, so a search must reach the same
   * server that indexed.
   */
  async index(request: ChunkRequest): Promise<number> {
    const body = await this.send<{ count: number }>("POST", "/v1/index", request);
    return body.count;
  }

  /** `POST /v1/search`: search that index and return the nearest chunks. */
  async search(request: SearchRequest): Promise<Hit[]> {
    const body = await this.send<{ hits: Hit[] }>("POST", "/v1/search", request);
    return body.hits;
  }

  /** `GET /v1/sources`: list the origins the server's index holds, most records first. */
  async listSources(): Promise<Source[]> {
    const body = await this.send<{ sources: Source[] }>("GET", "/v1/sources");
    return body.sources;
  }

  /**
   * `DELETE /v1/sources/{name}`: remove every chunk of one origin from
   * the server's index. A name the index does not hold deletes nothing
   * and resolves to 0 — deleting to zero is the normal end of a
   * re-index, not an error.
   */
  async deleteSource(name: string): Promise<number> {
    const body = await this.send<{ deleted: number }>("DELETE", `/v1/sources/${encodeURIComponent(name)}`);
    return body.deleted;
  }

  private async send<T>(method: string, path: string, payload?: unknown): Promise<T> {
    const init: {
      method: string;
      headers?: Record<string, string>;
      body?: string;
      signal?: AbortSignal;
    } = { method };

    // Only what was asked for is sent. `JSON.stringify` drops an undefined
    // field, so an omitted option stays omitted, and the server applies the
    // same default it applies to the CLI.
    if (payload !== undefined) {
      init.headers = { "content-type": "application/json", ...this.headers };
      init.body = JSON.stringify(payload);
    } else if (Object.keys(this.headers).length > 0) {
      init.headers = { ...this.headers };
    }
    if (this.timeoutMs > 0) {
      init.signal = AbortSignal.timeout(this.timeoutMs);
    }

    let response: Response;
    try {
      response = await this.fetchImpl(this.baseUrl + path, init);
    } catch (cause) {
      // A refused connection, a DNS failure, or the timeout above. The
      // request never reached an API, so there is no status to report.
      throw new NibbleError(0, `request to ${this.baseUrl}${path} failed: ${errorText(cause)}`);
    }

    const text = await response.text();
    if (!response.ok) {
      throw new NibbleError(response.status, apiError(response.status, text), text);
    }
    try {
      return JSON.parse(text) as T;
    } catch (cause) {
      throw new NibbleError(response.status, `response was not JSON: ${errorText(cause)}`, text);
    }
  }
}

/** The message the API sent, or a generic one when the body was not its JSON. */
function apiError(status: number, body: string): string {
  try {
    const parsed = JSON.parse(body) as { error?: unknown };
    if (typeof parsed.error === "string" && parsed.error !== "") {
      return parsed.error;
    }
  } catch {
    // Not JSON. Fall through to the status alone.
  }
  return `request failed with status ${status}`;
}

function errorText(cause: unknown): string {
  return cause instanceof Error ? cause.message : String(cause);
}
