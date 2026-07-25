/**
 * U-42 — k6 smoke load-test (NOT a full prod soak).
 *
 * Hits local/staging API liveness + readiness + a few public read routes under
 * light VU pressure. Use against a running API only (./run.sh or compose).
 *
 * Usage:
 *   k6 run deploy/load-test/smoke.js
 *   API_BASE=http://localhost:8090 k6 run deploy/load-test/smoke.js
 *   make load-test
 *
 * Honest limits: no auth journeys, no payment webhooks, no Arena WS, no soak
 * duration. Thresholds are "smoke not on fire", not capacity planning.
 */
import http from "k6/http";
import { check, group, sleep } from "k6";

const API_BASE = (__ENV.API_BASE || "http://localhost:8090").replace(/\/$/, "");

export const options = {
  scenarios: {
    smoke: {
      executor: "constant-vus",
      vus: Number(__ENV.VUS || 5),
      duration: __ENV.DURATION || "30s",
    },
  },
  thresholds: {
    http_req_failed: ["rate<0.05"],
    http_req_duration: ["p(95)<1500"],
    checks: ["rate>0.95"],
  },
};

function okJSON(res) {
  if (res.status !== 200) return false;
  try {
    const body = res.json();
    // Envelope shape from httpx: { data: … } or readiness { data: { status: "ok", … } }
    return body && typeof body === "object";
  } catch (_) {
    return false;
  }
}

export default function () {
  group("probes", () => {
    const health = http.get(`${API_BASE}/healthz`);
    check(health, {
      "healthz 200": (r) => r.status === 200,
      "healthz status ok": (r) => {
        try {
          return r.json("data.status") === "ok";
        } catch (_) {
          return false;
        }
      },
    });

    const ready = http.get(`${API_BASE}/readyz`);
    check(ready, {
      "readyz 200": (r) => r.status === 200,
      "readyz status ok": (r) => {
        try {
          return r.json("data.status") === "ok";
        } catch (_) {
          return false;
        }
      },
    });

    // Prefer Prometheus text; also accept JSON when negotiated.
    const metrics = http.get(`${API_BASE}/metrics`, {
      headers: { Accept: "text/plain" },
    });
    check(metrics, {
      "metrics 200": (r) => r.status === 200,
      "metrics body non-empty": (r) => String(r.body || "").length > 0,
    });
  });

  group("public reads", () => {
    const paths = [
      "/api/v1/categories",
      "/api/v1/variants",
      "/api/v1/signs",
    ];
    for (const path of paths) {
      const res = http.get(`${API_BASE}${path}`, {
        headers: { Accept: "application/json" },
      });
      check(res, {
        [`${path} 200`]: (r) => r.status === 200,
        [`${path} json`]: (r) => okJSON(r),
      });
    }
  });

  sleep(0.5);
}
