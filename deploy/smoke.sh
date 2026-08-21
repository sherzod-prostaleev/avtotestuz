#!/usr/bin/env bash
# Post-deploy / local overlay smoke — dependency-light (curl + optional python).
# Usage: ./deploy/smoke.sh <api_base> [web_base]
# Example: ./deploy/smoke.sh http://localhost:8080 http://localhost:3000
set -euo pipefail

API_BASE="${1:-http://localhost:8080}"
WEB_BASE="${2:-}"

API_BASE="${API_BASE%/}"
WEB_BASE="${WEB_BASE%/}"

echo "==> healthz @ ${API_BASE}/healthz"
body="$(curl -fsS "${API_BASE}/healthz")"
echo "${body}"
case "${body}" in
  *'"status":"ok"'*|*'\"status\":\"ok\"'*) ;;
  *)
    # Tolerate compact JSON without spaces
    if ! printf '%s' "${body}" | grep -q '"status"[[:space:]]*:[[:space:]]*"ok"'; then
      echo "healthz: unexpected body" >&2
      exit 1
    fi
    ;;
esac

echo "==> readyz @ ${API_BASE}/readyz"
ready_body="$(curl -fsS "${API_BASE}/readyz")"
echo "${ready_body}"
if ! printf '%s' "${ready_body}" | grep -q '"status"[[:space:]]*:[[:space:]]*"ok"'; then
  echo "readyz: unexpected body (want status ok + dependency checks)" >&2
  exit 1
fi
# Staging/prod API always wires Postgres+Redis — fail loud if either is not ok.
if ! printf '%s' "${ready_body}" | grep -q '"postgres"[[:space:]]*:[[:space:]]*"ok"'; then
  echo "readyz: postgres check not ok" >&2
  exit 1
fi
if ! printf '%s' "${ready_body}" | grep -q '"redis"[[:space:]]*:[[:space:]]*"ok"'; then
  echo "readyz: redis check not ok" >&2
  exit 1
fi
if ! printf '%s' "${ready_body}" | grep -q '"object_storage"[[:space:]]*:[[:space:]]*"ok"'; then
  echo "readyz: private object storage check not ok" >&2
  exit 1
fi

# The Windows agent every classroom PC updates itself from. Nothing else in
# the pipeline has ever looked at this file: the container is healthy with a
# missing, empty or unstamped agent, and the first detector of a broken one
# was a driving school phoning in. The manifest is the API's own view of the
# bytes it will serve, so checking it here covers existence, a real version
# and a real digest in one request.
echo "==> station agent manifest @ ${API_BASE}/api/v1/b2b/stations/agent-manifest"
agent_body="$(curl -fsS "${API_BASE}/api/v1/b2b/stations/agent-manifest")"
echo "${agent_body}"
if ! printf '%s' "${agent_body}" | grep -qE '"version"[[:space:]]*:[[:space:]]*"[0-9]+\.[0-9]+\.[0-9]+"'; then
  echo "station agent: no N.N.N version — the image was built without backend/station/VERSION" >&2
  exit 1
fi
if ! printf '%s' "${agent_body}" | grep -qE '"sha256"[[:space:]]*:[[:space:]]*"[0-9a-f]{64}"'; then
  echo "station agent: no sha256 — installed PCs would refuse every update" >&2
  exit 1
fi
agent_version="$(printf '%s' "${agent_body}" | sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
echo "station agent: ${agent_version}"

if [[ -n "${WEB_BASE}" ]]; then
  echo "==> web shell @ ${WEB_BASE}/uz-Latn"
  code="$(curl -fsS -o /dev/null -w '%{http_code}' "${WEB_BASE}/uz-Latn")"
  if [[ "${code}" != "200" ]]; then
    echo "web: expected HTTP 200, got ${code}" >&2
    exit 1
  fi
  echo "web: HTTP ${code}"
fi

echo "smoke: ok"
