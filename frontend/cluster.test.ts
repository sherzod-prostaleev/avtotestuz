import { describe, expect, it, vi } from "vitest";
import { createRequire } from "node:module";

const require_ = createRequire(import.meta.url);
const { resolveWorkerCount, MAX_WORKERS, createSupervisor } = require_("./cluster.js");

describe("resolveWorkerCount", () => {
  it("falls back when unset, empty or unparseable", () => {
    for (const raw of [undefined, "", "   ", "abc", "NaN"]) {
      expect(resolveWorkerCount(raw, 2)).toBe(2);
    }
  });

  it("falls back on values that would silently disable or invert clustering", () => {
    // "0" is the shape an unset shell variable takes after ${VAR:-0}; treating
    // it as "zero workers" would boot a Next server that listens to nobody.
    for (const raw of ["0", "-1", "-8"]) {
      expect(resolveWorkerCount(raw, 3)).toBe(3);
    }
  });

  it("takes explicit counts", () => {
    expect(resolveWorkerCount("1", 4)).toBe(1);
    expect(resolveWorkerCount("3", 1)).toBe(3);
  });

  it("truncates fractions rather than forking a fractional worker", () => {
    expect(resolveWorkerCount("2.9", 1)).toBe(2);
  });

  it("clamps absurd counts so a typo cannot fork-bomb the box", () => {
    expect(resolveWorkerCount("999", 1)).toBe(MAX_WORKERS);
  });
});

/** Minimal stand-ins for node:cluster and process, so nothing is really forked. */
function harness({ workers = 2 } = {}) {
  const forked: Array<{ id: number; process: { kill: ReturnType<typeof vi.fn> } }> = [];
  const exitHandlers: Array<(worker: unknown, code: number, signal: string | null) => void> = [];
  const signalHandlers = new Map<string, () => void>();
  let nextId = 1;

  const fakeCluster = {
    fork: vi.fn(() => {
      const worker = { id: nextId++, process: { kill: vi.fn() } };
      forked.push(worker);
      return worker;
    }),
    on: vi.fn((event: string, handler: never) => {
      if (event === "exit") exitHandlers.push(handler);
    }),
    get workers() {
      return Object.fromEntries(forked.map((w) => [w.id, w]));
    },
  };

  const fakeProcess = {
    on: vi.fn((signal: string, handler: () => void) => {
      signalHandlers.set(signal, handler);
    }),
    exit: vi.fn(),
  };

  let now = 0;
  const supervisor = createSupervisor({
    cluster: fakeCluster,
    proc: fakeProcess,
    workers,
    now: () => now,
    log: () => {},
  });

  return {
    supervisor,
    fakeCluster,
    fakeProcess,
    forked,
    advance: (ms: number) => {
      now += ms;
    },
    exit: (code: number, signal: string | null = null) => {
      for (const handler of exitHandlers) handler({ id: 0, process: { kill: vi.fn() } }, code, signal);
    },
    signal: (name: string) => signalHandlers.get(name)?.(),
  };
}

describe("cluster supervisor", () => {
  it("forks exactly the requested number of workers", () => {
    const h = harness({ workers: 3 });
    h.supervisor.start();
    expect(h.fakeCluster.fork).toHaveBeenCalledTimes(3);
  });

  it("replaces a worker that dies unexpectedly", () => {
    const h = harness({ workers: 2 });
    h.supervisor.start();
    h.advance(60_000);
    h.exit(1);
    expect(h.fakeCluster.fork).toHaveBeenCalledTimes(3);
  });

  it("gives up instead of spinning when workers die immediately", () => {
    // A worker that cannot boot (bad env, missing build) would otherwise be
    // reforked thousands of times per second forever, pinning a core and
    // hiding the real error. Exiting hands the decision to Docker's restart
    // policy, which backs off and surfaces the crash.
    const h = harness({ workers: 1 });
    h.supervisor.start();
    for (let i = 0; i < 12; i++) {
      h.advance(100);
      h.exit(1);
    }
    expect(h.fakeProcess.exit).toHaveBeenCalledWith(1);
    expect(h.fakeCluster.fork.mock.calls.length).toBeLessThan(12);
  });

  it("forwards SIGTERM to every worker instead of leaving them to the grace timeout", () => {
    // Docker signals PID 1 only. Without this the workers keep serving until
    // stop_grace_period expires and every deploy costs 30s of hung sockets.
    const h = harness({ workers: 2 });
    h.supervisor.start();
    h.signal("SIGTERM");
    for (const worker of h.forked) {
      expect(worker.process.kill).toHaveBeenCalledWith("SIGTERM");
    }
  });

  it("does not refork workers once shutdown has begun", () => {
    const h = harness({ workers: 2 });
    h.supervisor.start();
    h.signal("SIGTERM");
    const afterSignal = h.fakeCluster.fork.mock.calls.length;
    h.exit(0, "SIGTERM");
    expect(h.fakeCluster.fork).toHaveBeenCalledTimes(afterSignal);
  });
});
