#!/usr/bin/env bash
# Validate an operator-collected Windows 7 station smoke-test evidence bundle.
# This is deliberately offline: CI validates evidence structure, while the
# checklist defines how each file is collected on a real Windows 7 machine.
set -euo pipefail

readonly FORMAT="avtotest-win7-smoke-v1"
readonly OWNED_ORIGIN="https://drivergo.uz"

fail() {
  printf 'Win7 smoke evidence: %s\n' "$*" >&2
  exit 1
}

usage() {
  printf 'usage: %s EVIDENCE_DIRECTORY\n' "$(basename "$0")" >&2
  exit 2
}

[[ "$#" -eq 1 ]] || usage
BUNDLE="$1"
[[ -d "$BUNDLE" ]] || fail "evidence directory does not exist: $BUNDLE"

require_regular_file() {
  local path="$1"
  [[ ! -L "$path" && -f "$path" ]] || fail "required regular file is missing: $(basename "$path")"
  [[ -s "$path" ]] || fail "required evidence file is empty: $(basename "$path")"
}

for required in manifest.env selftest.txt dpapi-cross-machine.txt registry.txt owned-https.txt authenticode.txt; do
  require_regular_file "$BUNDLE/$required"
done

declare -A manifest=()
while IFS='=' read -r key value || [[ -n "$key" || -n "$value" ]]; do
  [[ "$key" =~ ^(format|station_version|operator|tested_at_utc|owned_origin)$ ]] ||
    fail "manifest has an unknown or malformed key: $key"
  [[ -n "$value" && -z "${manifest[$key]+set}" ]] ||
    fail "manifest has an empty or duplicate value for: $key"
  manifest["$key"]="$value"
done <"$BUNDLE/manifest.env"

for key in format station_version operator tested_at_utc owned_origin; do
  [[ -n "${manifest[$key]:-}" ]] || fail "manifest is missing: $key"
done
[[ "${manifest[format]}" == "$FORMAT" ]] || fail "unsupported manifest format"
[[ "${manifest[station_version]}" =~ ^[0-9A-Za-z][0-9A-Za-z._+-]*$ ]] ||
  fail "station_version is malformed"
[[ "${manifest[operator]}" =~ ^[^[:cntrl:]]+$ ]] || fail "operator is malformed"
[[ "${manifest[tested_at_utc]}" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]] ||
  fail "tested_at_utc must be UTC RFC3339 seconds"
[[ "${manifest[owned_origin]}" == "$OWNED_ORIGIN" ]] ||
  fail "owned_origin is not the approved station origin"

for check in {1..7}; do
  grep -Eq "^\[$check/7\].* PASS$" "$BUNDLE/selftest.txt" ||
    fail "selftest is missing PASS for check $check of 7"
done
grep -Fq 'SELFTEST RESULT: PASS' "$BUNDLE/selftest.txt" ||
  fail "selftest did not report PASS"

grep -Eq '^source_machine=[^[:space:]]+$' "$BUNDLE/dpapi-cross-machine.txt" ||
  fail "cross-machine evidence is missing source_machine"
grep -Eq '^destination_machine=[^[:space:]]+$' "$BUNDLE/dpapi-cross-machine.txt" ||
  fail "cross-machine evidence is missing destination_machine"
source_machine="$(awk -F= '$1 == "source_machine" { print $2; exit }' "$BUNDLE/dpapi-cross-machine.txt")"
destination_machine="$(awk -F= '$1 == "destination_machine" { print $2; exit }' "$BUNDLE/dpapi-cross-machine.txt")"
[[ "$source_machine" != "$destination_machine" ]] ||
  fail "cross-machine evidence names the same source and destination"
grep -Fq 'RESULT: correctly bound to its original machine' "$BUNDLE/dpapi-cross-machine.txt" ||
  fail "cross-machine DPAPI binding did not pass"
if grep -Fq 'RESULT: SECURITY FAILURE' "$BUNDLE/dpapi-cross-machine.txt"; then
  fail "cross-machine evidence reports a DPAPI security failure"
fi

grep -Eq 'HKEY_LOCAL_MACHINE\\SOFTWARE\\Microsoft\\Cryptography' "$BUNDLE/registry.txt" ||
  fail "registry evidence lacks the MachineGuid key path"
grep -Eq 'MachineGuid[[:space:]]+REG_SZ[[:space:]]+[^[:space:]]+' "$BUNDLE/registry.txt" ||
  fail "registry evidence lacks a MachineGuid value"
grep -Eq '^\[1/7\].*Hardware id \(hwid\.Collect\).* PASS$' "$BUNDLE/selftest.txt" ||
  fail "selftest did not pass the registry-backed hardware-id check"

grep -Fqx "origin=$OWNED_ORIGIN" "$BUNDLE/owned-https.txt" ||
  fail "HTTPS evidence does not name the approved origin"
grep -Fqx 'tls_certificate_validation=PASS' "$BUNDLE/owned-https.txt" ||
  fail "HTTPS evidence did not validate the certificate"
grep -Eq '^http_status=2[0-9]{2}$' "$BUNDLE/owned-https.txt" ||
  fail "HTTPS evidence did not record a successful response"
grep -Fqx 'station_proxy=PASS' "$BUNDLE/owned-https.txt" ||
  fail "HTTPS evidence did not prove the station-owned connection path"

grep -Fqx 'signature_verification=PASS' "$BUNDLE/authenticode.txt" ||
  fail "Authenticode signature verification did not pass"
grep -Fqx 'timestamp_verification=PASS' "$BUNDLE/authenticode.txt" ||
  fail "Authenticode timestamp verification did not pass"

printf 'Win7 smoke evidence accepted: %s\n' "$BUNDLE"
