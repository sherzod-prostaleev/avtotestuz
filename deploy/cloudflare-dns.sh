#!/usr/bin/env bash
# Point drivergo.uz (and www) at this VPS via Cloudflare API (orange-cloud proxy).
#
# Usage:
#   export CLOUDFLARE_API_TOKEN='...'   # Zone.DNS Edit for drivergo.uz
#   ./deploy/cloudflare-dns.sh
#
# Optional:
#   CF_ZONE_NAME=drivergo.uz ORIGIN_IP=89.117.59.137 ./deploy/cloudflare-dns.sh
set -euo pipefail

ZONE_NAME="${CF_ZONE_NAME:-drivergo.uz}"
ORIGIN_IP="${ORIGIN_IP:-89.117.59.137}"
TOKEN="${CLOUDFLARE_API_TOKEN:-${CF_API_TOKEN:-}}"

if [[ -z "${TOKEN}" ]]; then
  echo "Set CLOUDFLARE_API_TOKEN (Zone → DNS → Edit) and re-run." >&2
  exit 2
fi

api() {
  local method="$1" path="$2"
  shift 2
  curl -fsS -X "${method}" "https://api.cloudflare.com/client/v4${path}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    "$@"
}

zone_id="$(api GET "/zones?name=${ZONE_NAME}" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["result"][0]["id"])')"
echo "zone_id=${zone_id}"

upsert_a() {
  local name="$1"
  local existing
  existing="$(api GET "/zones/${zone_id}/dns_records?type=A&name=${name}" \
    | python3 -c 'import json,sys; r=json.load(sys.stdin)["result"]; print(r[0]["id"] if r else "")')"
  local body
  body="$(python3 - <<PY
import json
print(json.dumps({
  "type": "A",
  "name": "${name}",
  "content": "${ORIGIN_IP}",
  "ttl": 1,
  "proxied": True,
}))
PY
)"
  if [[ -n "${existing}" ]]; then
    echo "Updating A ${name} → ${ORIGIN_IP} (proxied)"
    api PUT "/zones/${zone_id}/dns_records/${existing}" --data "${body}" >/dev/null
  else
    echo "Creating A ${name} → ${ORIGIN_IP} (proxied)"
    api POST "/zones/${zone_id}/dns_records" --data "${body}" >/dev/null
  fi
}

upsert_a "${ZONE_NAME}"
upsert_a "www.${ZONE_NAME}"

echo "DNS upserted. Install the origin certificate, then require Cloudflare SSL/TLS Full (strict)."
echo "Smoke: curl -fsS https://${ZONE_NAME}/healthz"
