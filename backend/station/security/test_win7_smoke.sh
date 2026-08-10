#!/usr/bin/env bash
# Offline contract test for the operator-supplied Win7 smoke evidence bundle.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HARNESS="$SCRIPT_DIR/run_win7_smoke.sh"
TEST_ROOT="$(mktemp -d /tmp/avtotest-win7-smoke-test.XXXXXX)"

cleanup() {
  rm -rf -- "$TEST_ROOT"
}
trap cleanup EXIT

fail() {
  printf 'not ok - %s\n' "$*" >&2
  exit 1
}

pass_count=0
pass() {
  pass_count="$((pass_count + 1))"
  printf 'ok %d - %s\n' "$pass_count" "$*"
}

create_valid_bundle() {
  local bundle="$1"
  mkdir -p "$bundle"
  cat >"$bundle/manifest.env" <<'EOF'
format=avtotest-win7-smoke-v1
station_version=1.0.0
operator=fixture-operator
tested_at_utc=2026-08-10T06:00:00Z
owned_origin=https://drivergo.uz
EOF
  cat >"$bundle/selftest.txt" <<'EOF'
[1/7] Hardware id (hwid.Collect)                PASS
[2/7] Seal round-trip (keystore.Store)          PASS
[3/7] Key file is genuinely sealed on disk      PASS
[4/7] Tamper rejection                          PASS
[5/7] Empty file rejection                      PASS
[6/7] Autostart round-trip (selfinstall)        PASS
[7/7] Install target writable (selfinstall)     PASS
SELFTEST RESULT: PASS
EOF
  cat >"$bundle/dpapi-cross-machine.txt" <<'EOF'
source_machine=WIN7-FIXTURE-A
destination_machine=WIN7-FIXTURE-B
RESULT: correctly bound to its original machine
EOF
  cat >"$bundle/registry.txt" <<'EOF'
HKEY_LOCAL_MACHINE\SOFTWARE\Microsoft\Cryptography
    MachineGuid    REG_SZ    fixture-machine-guid
EOF
  cat >"$bundle/owned-https.txt" <<'EOF'
origin=https://drivergo.uz
tls_certificate_validation=PASS
http_status=200
station_proxy=PASS
EOF
  cat >"$bundle/authenticode.txt" <<'EOF'
signature_verification=PASS
timestamp_verification=PASS
EOF
}

if [[ ! -x "$HARNESS" ]]; then
  fail "harness is missing or not executable: $HARNESS"
fi

VALID="$TEST_ROOT/valid"
create_valid_bundle "$VALID"
"$HARNESS" "$VALID" >/dev/null
pass "complete evidence bundle is accepted"

MISSING="$TEST_ROOT/missing"
create_valid_bundle "$MISSING"
rm -- "$MISSING/owned-https.txt"
if "$HARNESS" "$MISSING" >/dev/null 2>&1; then
  fail "missing owned HTTPS evidence was accepted"
fi
pass "missing evidence fails closed"

UNSIGNED="$TEST_ROOT/unsigned"
create_valid_bundle "$UNSIGNED"
rm -- "$UNSIGNED/authenticode.txt"
if "$HARNESS" "$UNSIGNED" >/dev/null 2>&1; then
  fail "missing Authenticode verification evidence was accepted"
fi
pass "missing Authenticode evidence fails closed"

UNOWNED="$TEST_ROOT/unowned"
create_valid_bundle "$UNOWNED"
sed -i 's#https://drivergo\.uz#https://example.invalid#g' \
  "$UNOWNED/manifest.env" "$UNOWNED/owned-https.txt"
if "$HARNESS" "$UNOWNED" >/dev/null 2>&1; then
  fail "unowned HTTPS origin was accepted"
fi
pass "unowned HTTPS origin is rejected"

printf '1..%d\n' "$pass_count"
