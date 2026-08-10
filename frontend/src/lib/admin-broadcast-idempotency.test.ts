import { describe, expect, it, vi } from "vitest";

import { createBroadcastIdempotencySession } from "./admin-broadcast-idempotency";

describe("createBroadcastIdempotencySession", () => {
  it("mints a UUID once per logical send and reuses it on network retry", () => {
    const createKey = vi
      .fn<() => string>()
      .mockReturnValueOnce("key-a")
      .mockReturnValueOnce("key-b");

    const session = createBroadcastIdempotencySession(createKey);

    const first = session.begin();
    expect(first).toEqual({ key: "key-a", skip: false });
    expect(createKey).toHaveBeenCalledTimes(1);

    // Simulated network failure — keep key for retry.
    session.failKeepKey();
    const retry = session.begin();
    expect(retry).toEqual({ key: "key-a", skip: false });
    expect(createKey).toHaveBeenCalledTimes(1);
    expect(session.currentKey()).toBe("key-a");
  });

  it("blocks double-click / double-submit while a request is in flight", () => {
    const session = createBroadcastIdempotencySession(() => "same-key");

    const a = session.begin();
    const b = session.begin();
    expect(a).toEqual({ key: "same-key", skip: false });
    expect(b).toEqual({ key: "same-key", skip: true });
  });

  it("issues a new key only after a successful send", () => {
    const createKey = vi
      .fn<() => string>()
      .mockReturnValueOnce("key-1")
      .mockReturnValueOnce("key-2");
    const session = createBroadcastIdempotencySession(createKey);

    expect(session.begin().key).toBe("key-1");
    session.succeed();
    expect(session.currentKey()).toBeNull();

    expect(session.begin()).toEqual({ key: "key-2", skip: false });
    expect(createKey).toHaveBeenCalledTimes(2);
  });

  it("reset clears a stuck key so the next send is a new logical operation", () => {
    const createKey = vi
      .fn<() => string>()
      .mockReturnValueOnce("old")
      .mockReturnValueOnce("new");
    const session = createBroadcastIdempotencySession(createKey);

    session.begin();
    session.failKeepKey();
    session.reset();
    expect(session.begin()).toEqual({ key: "new", skip: false });
  });
});
