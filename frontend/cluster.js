"use strict";

/**
 * Multi-process entrypoint for the standalone Next server.
 *
 * Why this exists: JavaScript execution in Node is single-threaded, and every
 * authenticated call in this app goes browser -> nginx -> THIS process -> Go.
 * Measured on the production box (4 vCPU, container capped at 1.5), with the
 * Go API idling at 35% CPU the whole time:
 *
 *   Go API   /api/v1/flags     ~1500 rps
 *   BFF      /api/proxy/flags   ~195 rps   <- one JS thread, pegged
 *   SSR      /uz-Latn            ~45 rps   <- same thread
 *
 * The backend was never the constraint; one JS thread was. Roughly half of the
 * per-request cost is Next's own route-handler machinery (a route handler that
 * does nothing but return a literal still capped at ~327 rps), so it cannot be
 * optimised away in application code -- it has to be run on more than one core.
 *
 * cluster gives that with the primary owning the listening socket and
 * round-robining accepted connections, so nginx keeps talking to one port and
 * the healthcheck, keep-alive contract and container shape are unchanged.
 *
 * Safe to run multiple workers: the one piece of cross-request state that would
 * break when sharded is refresh-token single-flight (src/lib/refresh-lock.ts),
 * and that is only an optimisation. The authoritative protection is server-side
 * and shared -- Go keeps an encrypted 45s grace entry in Redis under
 * `auth:rtgrace:<hash>` plus a distributed claim, so two workers refreshing the
 * same token concurrently both receive the same successor pair rather than
 * tripping reuse detection. Go's 45s window is strictly longer than the Node
 * cache's 30s, so nothing is lost by missing the in-process hit.
 */

const cluster = require("node:cluster");

/** A typo in an env var must not be able to fork-bomb a 4-core box. */
const MAX_WORKERS = 8;

/** Rapid successive worker deaths that mean "this will never boot". */
const CRASH_WINDOW_MS = 10_000;
const CRASH_LIMIT = 5;

/**
 * Parse WEB_WORKERS. Anything that is not a usable positive count -- unset,
 * blank, non-numeric, zero, negative -- falls back rather than being coerced,
 * because every one of those shapes means "nobody chose a value" and the one
 * thing that must never happen is booting with zero workers.
 */
function resolveWorkerCount(raw, fallback) {
  const parsed = Number.parseInt(String(raw ?? "").trim(), 10);
  if (!Number.isFinite(parsed) || parsed < 1) return fallback;
  return Math.min(parsed, MAX_WORKERS);
}

function createSupervisor({ cluster: clu, proc, workers, now, log }) {
  let shuttingDown = false;
  const recentDeaths = [];

  function spawn() {
    clu.fork();
  }

  function onExit(_worker, code, signal) {
    if (shuttingDown) return;

    const at = now();
    recentDeaths.push(at);
    while (recentDeaths.length > 0 && at - recentDeaths[0] > CRASH_WINDOW_MS) {
      recentDeaths.shift();
    }
    if (recentDeaths.length >= CRASH_LIMIT) {
      log(
        `web: ${recentDeaths.length} worker deaths within ${CRASH_WINDOW_MS}ms; ` +
          `giving up so the restart policy can back off and surface the error`
      );
      proc.exit(1);
      return;
    }

    log(`web: worker exited (code=${code} signal=${signal}); replacing it`);
    spawn();
  }

  function shutdown(signal) {
    if (shuttingDown) return;
    shuttingDown = true;
    // Docker signals PID 1 only; cluster does not relay. Without this the
    // workers keep holding sockets until stop_grace_period expires.
    for (const worker of Object.values(clu.workers ?? {})) {
      worker?.process?.kill(signal);
    }
  }

  return {
    start() {
      clu.on("exit", onExit);
      for (const signal of ["SIGTERM", "SIGINT"]) {
        proc.on(signal, () => shutdown(signal));
      }
      for (let i = 0; i < workers; i += 1) spawn();
    },
  };
}

module.exports = { resolveWorkerCount, createSupervisor, MAX_WORKERS };

if (require.main === module) {
  const workers = resolveWorkerCount(process.env.WEB_WORKERS, 1);

  if (cluster.isPrimary && workers > 1) {
    console.log(`web: starting ${workers} workers`);
    createSupervisor({
      cluster,
      proc: process,
      workers,
      now: () => Date.now(),
      log: (message) => console.log(message),
    }).start();
  } else {
    // Single-worker (and every forked child) path: identical to the previous
    // `node server.js` entrypoint, so WEB_WORKERS=1 is a true no-op rollback.
    require("./server.js");
  }
}
