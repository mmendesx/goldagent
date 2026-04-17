import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { restClient, HttpError, TimeoutError, NetworkError } from "./restClient";

// ---------------------------------------------------------------------------
// Notes on isolation
// ---------------------------------------------------------------------------
// The `inFlight` dedup map is module-level state.  Each test below picks a
// distinct endpoint (metrics / positions / trades / exchange balances) so
// residual entries in `inFlight` from one describe block cannot leak into
// another.  Tests also fully exhaust retry chains using `runAllTimersAsync`
// so pending promises never leak across tests.
// ---------------------------------------------------------------------------

function mockFetchResponse(status: number, body: unknown = {}): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    text: async () => `status ${status}`,
    json: async () => body,
  } as unknown as Response;
}

// Returns a deferred fetch mock: resolve/reject on demand.
function deferredFetch(): {
  mock: ReturnType<typeof vi.fn>;
  resolve: (value: Response) => void;
  reject: (reason: unknown) => void;
} {
  let resolve!: (value: Response) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<Response>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  const mock = vi.fn(() => promise);
  return { mock, resolve, reject };
}

// A fetch mock that rejects when the supplied AbortSignal fires.
function abortableFetch(): ReturnType<typeof vi.fn> {
  return vi.fn((_url: string, init?: RequestInit) => {
    return new Promise<Response>((_resolve, reject) => {
      const signal = init?.signal as AbortSignal | undefined;
      const abortError = new DOMException("aborted", "AbortError");
      if (signal?.aborted) {
        reject(abortError);
        return;
      }
      signal?.addEventListener("abort", () => {
        reject(abortError);
      });
    });
  });
}

beforeEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

afterEach(() => {
  vi.useRealTimers();
});

// ---------------------------------------------------------------------------
// fetchWithTimeout — request() aborts via AbortController + setTimeout.  The
// restClient methods call withRetry, which will retry TimeoutError up to the
// retry cap.  We use runAllTimersAsync so every timeout and backoff fires.
// We use fetchExchangeBalances for isolation from the dedup tests (which
// target /api/v1/metrics).
// ---------------------------------------------------------------------------

describe("fetchWithTimeout", () => {
  it("rejects with TimeoutError when requests exceed the timeout", async () => {
    vi.useFakeTimers();
    vi.stubGlobal("fetch", abortableFetch());

    const resultPromise = restClient.fetchExchangeBalances();
    // Attach an early catch handler to avoid unhandled rejection warnings
    // while timer advancement pumps microtasks.
    const caught = resultPromise.catch((e: unknown) => e);

    await vi.runAllTimersAsync();

    const error = await caught;
    expect(error).toBeInstanceOf(TimeoutError);
  });

  it("TimeoutError message contains the request URL", async () => {
    vi.useFakeTimers();
    vi.stubGlobal("fetch", abortableFetch());

    const resultPromise = restClient.fetchExchangeBalances();
    const caught = resultPromise.catch((e: unknown) => e);

    await vi.runAllTimersAsync();

    const error = (await caught) as Error;
    expect(error.message).toContain("http://localhost:8080");
    expect(error.message).toContain("/api/v1/exchange/balances");
  });
});

// ---------------------------------------------------------------------------
// withRetry — exercised indirectly through restClient methods.
// ---------------------------------------------------------------------------

describe("withRetry", () => {
  it("retries once after a 503 and returns the 200 payload", async () => {
    vi.useFakeTimers();
    const payload = [{ id: 1, symbol: "BTCUSDT" }];
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(mockFetchResponse(503))
      .mockResolvedValueOnce(mockFetchResponse(200, payload));
    vi.stubGlobal("fetch", fetchMock);

    const resultPromise = restClient.fetchOpenPositions();

    // Flush the backoff delay between attempts.
    await vi.runAllTimersAsync();

    const result = await resultPromise;
    expect(result).toEqual(payload);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });

  it("does not retry a 400 response and throws HttpError immediately", async () => {
    const fetchMock = vi.fn().mockResolvedValue(mockFetchResponse(400));
    vi.stubGlobal("fetch", fetchMock);

    await expect(restClient.fetchOpenPositions()).rejects.toBeInstanceOf(
      HttpError,
    );

    // Must have been called exactly once — no retry attempted on 4xx.
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("throws HttpError carrying the 4xx status code", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(mockFetchResponse(400)),
    );

    const error = await restClient
      .fetchOpenPositions()
      .catch((e: unknown) => e);
    expect(error).toBeInstanceOf(HttpError);
    expect((error as HttpError).status).toBe(400);
  });

  it("exhausts all 3 attempts on repeated 503 and throws the final HttpError", async () => {
    vi.useFakeTimers();
    // retries = 2 default => 1 initial + 2 retries = 3 attempts total.
    const fetchMock = vi.fn().mockResolvedValue(mockFetchResponse(503));
    vi.stubGlobal("fetch", fetchMock);

    const resultPromise = restClient.fetchTrades();
    const caught = resultPromise.catch((e: unknown) => e);

    await vi.runAllTimersAsync();

    const error = await caught;
    expect(error).toBeInstanceOf(HttpError);
    expect((error as HttpError).status).toBe(503);
    expect(fetchMock).toHaveBeenCalledTimes(3);
  });

  it("succeeds without retrying when the first response is 200", async () => {
    const payload = [{ id: 1 }];
    const fetchMock = vi
      .fn()
      .mockResolvedValue(mockFetchResponse(200, payload));
    vi.stubGlobal("fetch", fetchMock);

    const result = await restClient.fetchOpenPositions();
    expect(result).toEqual(payload);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});

// ---------------------------------------------------------------------------
// Deduplication — two concurrent calls with the same dedup key must share
// a single fetch invocation and both receive the same result.  Uses
// /api/v1/metrics so it cannot collide with other tests.
// ---------------------------------------------------------------------------

describe("dedup", () => {
  it("two concurrent fetchMetrics calls share a single fetch invocation", async () => {
    const payload = { totalValue: "9999" };
    const { mock, resolve } = deferredFetch();
    vi.stubGlobal("fetch", mock);

    const first = restClient.fetchMetrics();
    const second = restClient.fetchMetrics();

    // Both calls dispatched before any response — fetch must have been
    // invoked exactly once.
    expect(mock).toHaveBeenCalledTimes(1);

    // Settle the in-flight request so both callers receive the same payload.
    resolve(mockFetchResponse(200, payload));

    const [resultA, resultB] = await Promise.all([first, second]);
    expect(resultA).toEqual(payload);
    expect(resultB).toEqual(payload);
  });

  it("a second fetchMetrics after the first settles issues a new fetch", async () => {
    const payload = { totalValue: "42" };
    const fetchMock = vi
      .fn()
      .mockResolvedValue(mockFetchResponse(200, payload));
    vi.stubGlobal("fetch", fetchMock);

    await restClient.fetchMetrics();
    await restClient.fetchMetrics();

    // Two separate calls, each after the previous settled — two fetches.
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
});

// ---------------------------------------------------------------------------
// Error class invariants
// ---------------------------------------------------------------------------

describe("HttpError", () => {
  it("carries the HTTP status code", () => {
    const err = new HttpError(422, "unprocessable");
    expect(err.status).toBe(422);
    expect(err.name).toBe("HttpError");
    expect(err).toBeInstanceOf(Error);
  });
});

describe("TimeoutError", () => {
  it("includes the URL in the message", () => {
    const err = new TimeoutError("http://example.com/api");
    expect(err.message).toContain("http://example.com/api");
    expect(err.name).toBe("TimeoutError");
    expect(err).toBeInstanceOf(Error);
  });
});

describe("NetworkError", () => {
  it("preserves the original message", () => {
    const err = new NetworkError("ECONNREFUSED");
    expect(err.message).toBe("ECONNREFUSED");
    expect(err.name).toBe("NetworkError");
    expect(err).toBeInstanceOf(Error);
  });
});
