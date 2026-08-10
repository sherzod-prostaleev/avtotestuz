#!/usr/bin/env bash
# Exercise nginx include rollback with fake host commands only.
set -euo pipefail

if [[ "$EUID" -ne 0 && "${DRIVERGO_SWITCH_APP_SLOT_TEST_USERNS:-}" != 1 ]]; then
  exec unshare -Ur -- env DRIVERGO_SWITCH_APP_SLOT_TEST_USERNS=1 bash "$0" "$@"
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/deploy/switch-app-slot.sh"
test_root="$(mktemp -d)"
trap 'rm -rf -- "$test_root"' EXIT

mkdir -p "$test_root/bin" "$test_root/nginx"
active_include="$test_root/nginx/drivergo-upstreams.conf"
stable_include="$ROOT/deploy/nginx/upstreams-stable.conf"
candidate_include="$ROOT/deploy/nginx/upstreams-candidate.conf"
cp -- "$stable_include" "$active_include"

cat >"$test_root/bin/curl" <<'EOF'
#!/usr/bin/env bash
case "$*" in
  *readyz*) printf '{"status":"ok"}\n' ;;
esac
EOF
cat >"$test_root/bin/nginx" <<'EOF'
#!/usr/bin/env bash
if [[ "${DRIVERGO_NGINX_TEST_MODE:-}" == wait ]]; then
  : "${DRIVERGO_NGINX_TEST_MARKER:?}"
  : >"$DRIVERGO_NGINX_TEST_MARKER"
  sleep 1
fi
exit 0
EOF
cat >"$test_root/bin/systemctl" <<'EOF'
#!/usr/bin/env bash
[[ -z "${DRIVERGO_SYSTEMCTL_TEST_LOG:-}" ]] || printf '%s\n' "$*" >>"$DRIVERGO_SYSTEMCTL_TEST_LOG"
if [[ -n "${DRIVERGO_SYSTEMCTL_FAIL_ONCE_FILE:-}" && ! -e "$DRIVERGO_SYSTEMCTL_FAIL_ONCE_FILE" ]]; then
  : >"$DRIVERGO_SYSTEMCTL_FAIL_ONCE_FILE"
  exit 1
fi
exit "${DRIVERGO_SYSTEMCTL_EXIT:-0}"
EOF
chmod 0755 "$test_root/bin/"{curl,nginx,systemctl}

run_switch() {
  env \
    PATH="$test_root/bin:$PATH" \
    DRIVERGO_NGINX_UPSTREAM_INCLUDE="$active_include" \
    DRIVERGO_SLOT_LOCK_FILE="$test_root/drivergo-app-slot.lock" \
    "$SCRIPT" --to candidate --apply
}

expect_previous_include() {
  cmp -s -- "$active_include" "$stable_include" ||
    { echo "previous nginx include was not restored" >&2; exit 1; }
  compgen -G "$test_root/nginx/.drivergo-upstreams.backup.*" >/dev/null ||
    { echo "rollback snapshot was not retained" >&2; exit 1; }
  ! compgen -G "$test_root/nginx/.drivergo-upstreams.candidate.*" >/dev/null ||
    { echo "candidate tempfile was not removed" >&2; exit 1; }
}

# A failed reload must restore the prior include and keep its snapshot.
reload_log="$test_root/nginx-reloads.log"
if DRIVERGO_SYSTEMCTL_FAIL_ONCE_FILE="$test_root/first-reload-failed" \
  DRIVERGO_SYSTEMCTL_TEST_LOG="$reload_log" run_switch; then
  echo "expected nginx reload failure" >&2
  exit 1
fi
expect_previous_include
[[ "$(wc -l <"$reload_log")" -eq 2 ]] ||
  { echo "previous nginx include was not reloaded after failure" >&2; exit 1; }

# A successful reload commits the new include and releases the snapshot.
rm -f -- "$test_root/nginx/.drivergo-upstreams.backup."*
cp -- "$stable_include" "$active_include"
run_switch
cmp -s -- "$active_include" "$candidate_include" ||
  { echo "candidate nginx include was not committed" >&2; exit 1; }
! compgen -G "$test_root/nginx/.drivergo-upstreams.backup.*" >/dev/null ||
  { echo "rollback snapshot survived a committed reload" >&2; exit 1; }

# Reset the fake active include before testing asynchronous interruption.
cp -- "$stable_include" "$active_include"
marker="$test_root/nginx-validation-started"
signal_reload_log="$test_root/nginx-reloads-after-signal.log"

# A signal after staging the new include must roll it back before exit.
env \
  PATH="$test_root/bin:$PATH" \
  DRIVERGO_NGINX_UPSTREAM_INCLUDE="$active_include" \
  DRIVERGO_SLOT_LOCK_FILE="$test_root/drivergo-app-slot.lock" \
  DRIVERGO_NGINX_TEST_MODE=wait \
  DRIVERGO_NGINX_TEST_MARKER="$marker" \
  DRIVERGO_SYSTEMCTL_TEST_LOG="$signal_reload_log" \
  "$SCRIPT" --to candidate --apply >"$test_root/switch.out" 2>"$test_root/switch.err" &
switch_pid=$!
for _ in {1..50}; do
  [[ -e "$marker" ]] && break
  sleep 0.1
done
[[ -e "$marker" ]] || { echo "switch never reached nginx validation" >&2; exit 1; }
kill -TERM "$switch_pid"
if wait "$switch_pid"; then
  echo "expected switch to be interrupted" >&2
  exit 1
fi
expect_previous_include
[[ "$(wc -l <"$signal_reload_log")" -eq 1 ]] ||
  { echo "previous nginx include was not reloaded after signal" >&2; exit 1; }

if cmp -s -- "$candidate_include" "$stable_include"; then
  { echo "test fixtures must use distinct upstream includes" >&2; exit 1; }
fi
printf 'switch-app-slot rollback guards: ok\n'
