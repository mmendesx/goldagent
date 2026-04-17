import { describe, it, expect, vi, afterEach } from "vitest";
import { WebSocketClient } from "./websocketClient";

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("WebSocketClient reconnect guard", () => {
  it("connect() while OPEN returns early without creating a new socket", () => {
    // Build a mock constructor that returns an OPEN-state socket
    const mockSocket = { readyState: 1 } as WebSocket;

    function MockWS() {
      return mockSocket;
    }
    // Attach static readyState constants so source code checks like WebSocket.OPEN resolve
    Object.assign(MockWS, { CONNECTING: 0, OPEN: 1, CLOSING: 2, CLOSED: 3 });
    const constructorSpy = vi.fn(MockWS);

    vi.stubGlobal("WebSocket", constructorSpy);

    const client = new WebSocketClient();
    // Force internal socket to be the already-open mock
    (client as any).socket = mockSocket;

    const callsBefore = constructorSpy.mock.calls.length;
    client.connect(); // should return early — socket is OPEN
    expect(constructorSpy.mock.calls.length).toBe(callsBefore); // no new socket created
  });

  it("connect() while CONNECTING returns early without creating a new socket", () => {
    const mockSocket = { readyState: 0 } as WebSocket; // CONNECTING

    function MockWS() {
      return mockSocket;
    }
    Object.assign(MockWS, { CONNECTING: 0, OPEN: 1, CLOSING: 2, CLOSED: 3 });
    const constructorSpy = vi.fn(MockWS);

    vi.stubGlobal("WebSocket", constructorSpy);

    const client = new WebSocketClient();
    (client as any).socket = mockSocket;

    const callsBefore = constructorSpy.mock.calls.length;
    client.connect();
    expect(constructorSpy.mock.calls.length).toBe(callsBefore);
  });

  it("connect() clears pending reconnectTimer before opening", () => {
    // In node environment, window.clearTimeout is not available; stub it globally
    const clearTimeoutSpy = vi.fn();
    vi.stubGlobal("window", {
      clearTimeout: clearTimeoutSpy,
      setTimeout: vi.fn(() => 0),
    });

    const mockSocket = {
      readyState: 3, // CLOSED
      onopen: null,
      onmessage: null,
      onerror: null,
      onclose: null,
    } as unknown as WebSocket;

    function MockWS() {
      return mockSocket;
    }
    Object.assign(MockWS, { CONNECTING: 0, OPEN: 1, CLOSING: 2, CLOSED: 3 });

    vi.stubGlobal("WebSocket", vi.fn(MockWS));

    const client = new WebSocketClient();
    (client as any).reconnectTimer = 99; // simulate a pending timer

    client.connect();

    expect(clearTimeoutSpy).toHaveBeenCalledWith(99);
  });

  it("reconnectTimer is nulled after clearTimeout on connect()", () => {
    vi.stubGlobal("window", {
      clearTimeout: vi.fn(),
      setTimeout: vi.fn(() => 0),
    });

    const mockSocket = {
      readyState: 3,
      onopen: null,
      onmessage: null,
      onerror: null,
      onclose: null,
    } as unknown as WebSocket;

    function MockWS() {
      return mockSocket;
    }
    Object.assign(MockWS, { CONNECTING: 0, OPEN: 1, CLOSING: 2, CLOSED: 3 });

    vi.stubGlobal("WebSocket", vi.fn(MockWS));

    const client = new WebSocketClient();
    (client as any).reconnectTimer = 99;

    client.connect();

    expect((client as any).reconnectTimer).toBeNull();
  });
});
