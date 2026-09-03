#!/usr/bin/env bash
# Turn Encrypted ClientHello (ECH) off for the zone, and verify it is gone.
#
# Why this exists
# ---------------
# Cloudflare advertises ECH in the zone's DNS HTTPS record:
#
#   drivergo.uz. HTTPS 1 . alpn="h3,h2" ech=AEX+DQBB... ipv4hint=...
#
# Firefox honours it and encrypts the ClientHello; Chrome does not. Uzbek ISP
# DPI resets the connection when it cannot read the server name, so Firefox
# users get PR_CONNECT_RESET_ERROR while Chrome loads the site normally.
# Confirmed on 2026-09-03: setting Firefox's `network.dns.echconfig.enabled`
# to false made the site load again on the same machine and network.
#
# Turning ECH off removes `ech=` from the record, so Firefox stops sending an
# encrypted ClientHello and the DPI stops resetting the handshake. TLS 1.3, the
# certificate and HSTS are unaffected — this only stops the server *name* being
# encrypted, which is already the case for every Chrome visitor.
#
# The setting is not exposed in the dashboard on this plan; the API is the only
# way to reach it. Cloudflare has flipped ECH on for zones by itself before, so
# treat this as a thing that can come back rather than a one-off.
#
# How it was actually applied on 2026-09-03
# -----------------------------------------
# Through the dashboard's own session, not this script: the dashboard is
# already authenticated, and `dash.cloudflare.com/api/v4/zones/<id>/settings/ech`
# accepts a PATCH of {"value":"off"} from a logged-in tab with no token at all.
# That is the quickest route if you have the dashboard open.
#
# Usage
# -----
#   export CLOUDFLARE_API_TOKEN='...'
#   ./deploy/cloudflare-ech-off.sh
#
#   ./deploy/cloudflare-ech-off.sh --status   # read the current value only
#   ./deploy/cloudflare-ech-off.sh --list     # every zone setting, to see if
#                                             # `ech` is missing or just denied
#
# The token: "Create Custom Token", NOT the "Edit zone settings" template.
# The template was tried twice on 2026-09-03 and both tokens got 403 on
# GET /zones/<id>/settings — the whole settings list, not only `ech`, which is
# what tells you it is a permission problem and not the plan. Grant explicitly:
#
#   Zone → Zone Settings → Edit
#   Zone → Zone          → Read
#   Zone Resources: Include → Specific zone → drivergo.uz
#
# Delete the token afterwards.
set -euo pipefail

ZONE_NAME="${CF_ZONE_NAME:-drivergo.uz}"
TOKEN="${CLOUDFLARE_API_TOKEN:-${CF_API_TOKEN:-}}"
MODE="${1:-apply}"

if [[ -z "${TOKEN}" ]]; then
  echo "Set CLOUDFLARE_API_TOKEN (Zone → Zone Settings → Edit) and re-run." >&2
  exit 2
fi

# No `-f`: on an error Cloudflare puts the reason in the body, and swallowing
# it leaves you staring at a bare "403" with nothing to act on.
api() {
  local method="$1" path="$2"
  shift 2
  local out status
  out="$(curl -sS -w '\n%{http_code}' -X "${method}" \
    "https://api.cloudflare.com/client/v4${path}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    "$@")"
  status="${out##*$'\n'}"
  body="${out%$'\n'*}"
  if [[ "${status}" != 2* ]]; then
    printf 'HTTP %s from %s %s\n' "${status}" "${method}" "${path}" >&2
    printf '%s\n' "${body}" | python3 -m json.tool 2>/dev/null || printf '%s\n' "${body}" >&2
    return 1
  fi
  printf '%s' "${body}"
}

zone_id="$(api GET "/zones?name=${ZONE_NAME}" \
  | python3 -c 'import json,sys; d=json.load(sys.stdin); r=d["result"]; print(r[0]["id"] if r else "")')"
[[ -n "${zone_id}" ]] || { echo "zone ${ZONE_NAME} not found for this token" >&2; exit 3; }
echo "zone: ${ZONE_NAME} (${zone_id})"

# `--list` answers the question a bare 403 cannot: is `ech` missing from this
# zone's settings (plan/entitlement), or is the token simply not allowed to
# read it (permissions)?
if [[ "${MODE}" == "--list" ]]; then
  api GET "/zones/${zone_id}/settings" > /tmp/cf-settings.json
  python3 - /tmp/cf-settings.json <<'PYEOF'
import json, sys
rows = json.load(open(sys.argv[1]))["result"]
for row in sorted(rows, key=lambda r: r["id"]):
    print("%-32s %-14s editable=%s" % (row["id"], repr(row.get("value"))[:14], row.get("editable")))
print()
print("ech present:", any(r["id"] == "ech" for r in rows))
PYEOF
  rm -f /tmp/cf-settings.json
  exit 0
fi

current="$(api GET "/zones/${zone_id}/settings/ech" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["result"]["value"])')"
echo "ech (before): ${current}"

if [[ "${MODE}" == "--status" ]]; then
  exit 0
fi

if [[ "${current}" == "off" ]]; then
  echo "already off; nothing to change"
else
  after="$(api PATCH "/zones/${zone_id}/settings/ech" --data '{"value":"off"}' \
    | python3 -c 'import json,sys; print(json.load(sys.stdin)["result"]["value"])')"
  echo "ech (after):  ${after}"
fi

# The HTTPS record is regenerated at the edge; give resolvers a moment, then
# show what the world actually sees. `ech=` disappearing is the real proof.
echo
echo "waiting for the HTTPS record to catch up..."
for _ in $(seq 1 12); do
  sleep 10
  rr="$(dig +short TYPE65 "${ZONE_NAME}" 2>/dev/null | head -1)"
  if [[ -n "${rr}" && "${rr}" != *"ech="* ]]; then
    echo "HTTPS record: ${rr}"
    echo "done: ech= is gone — Firefox will connect normally."
    exit 0
  fi
done

echo "HTTPS record still shows ech= after two minutes:" >&2
dig +short TYPE65 "${ZONE_NAME}" | head -1 >&2
echo "The setting is saved; resolvers cache this record, so re-check with:" >&2
echo "  dig +short TYPE65 ${ZONE_NAME}" >&2
