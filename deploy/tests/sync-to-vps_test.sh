#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SYNC_TO_VPS_SOURCE_ONLY=1 source "$ROOT/deploy/sync-to-vps.sh"

expect_reject() {
  if "$@"; then
    printf 'expected rejection: %q ' "$@" >&2
    printf '\n' >&2
    exit 1
  fi
}

validate_host root@89.117.59.137 "root@89.117.59.137 deploy@staging.example"
validate_host deploy@staging.example "root@89.117.59.137 deploy@staging.example"
expect_reject validate_host root@evil.example "root@89.117.59.137"
expect_reject validate_host '-oProxyCommand=evil' "-oProxyCommand=evil"

validate_path /opt/drivergo "/opt/drivergo /opt/drivergo-staging"
validate_path /opt/drivergo-staging "/opt/drivergo /opt/drivergo-staging"
expect_reject validate_path / "/"
expect_reject validate_path /opt "/opt"
expect_reject validate_path /opt/drivergo/../other "/opt/drivergo/../other"
expect_reject validate_path /var/lib/docker "/var/lib/docker"
expect_reject validate_path /opt/not-allowlisted "/opt/drivergo"

grep -Fq -- "--filter='P deploy/app.env'" "$ROOT/deploy/sync-to-vps.sh"
grep -Fq -- "--filter='P .drivergo-deploy-root'" "$ROOT/deploy/sync-to-vps.sh"
grep -Fq -- '--exclude-from="$dest/deploy/rsync-exclude.txt"' "$ROOT/deploy/sync-to-vps.sh"
grep -Fq -- '--exclude-from="$exclude_file"' "$ROOT/deploy/sync-to-vps.sh"
grep -Fq -- '--exclude-from="$current_exclude"' "$ROOT/deploy/sync-to-vps.sh"
grep -Fq -- 'StrictHostKeyChecking=yes' "$ROOT/deploy/sync-to-vps.sh"
grep -Fq 'readlink -f -- "$dest"' "$ROOT/deploy/sync-to-vps.sh"
grep -Fq 'partial_root="$rollback_root/.partial-$snapshot_id"' "$ROOT/deploy/sync-to-vps.sh"
grep -Fq 'mv -- "$partial_root" "$snapshot_root"' "$ROOT/deploy/sync-to-vps.sh"
grep -Fq 'complete="$snapshot_root/COMPLETE"' "$ROOT/deploy/sync-to-vps.sh"
grep -Fq 'rollback snapshot is incomplete' "$ROOT/deploy/sync-to-vps.sh"
grep -Fq 'http://127.0.0.1:3000/api/healthz' "$ROOT/deploy/sync-to-vps.sh"
grep -Fq 'http://127.0.0.1:3000/uz-Latn' "$ROOT/deploy/sync-to-vps.sh"

test_root="$(mktemp -d)"
trap 'rm -rf -- "$test_root"' EXIT
mkdir -p "$test_root/snapshot/deploy" "$test_root/target/backups"
cp "$ROOT/deploy/rsync-exclude.txt" "$test_root/snapshot/deploy/rsync-exclude.txt"
cp "$ROOT/deploy/rsync-exclude.txt" "$test_root/target/deploy-current-exclude.txt"
printf 'future-runtime/\n' >>"$test_root/target/deploy-current-exclude.txt"
printf 'tracked\n' >"$test_root/snapshot/tracked.txt"
printf 'must-survive\n' >"$test_root/target/backups/database.dump"
mkdir -p "$test_root/target/future-runtime"
printf 'new-protected-state\n' >"$test_root/target/future-runtime/state"
printf 'stale\n' >"$test_root/target/stale.txt"
rsync -a --delete \
  --exclude-from="$test_root/snapshot/deploy/rsync-exclude.txt" \
  --exclude-from="$test_root/target/deploy-current-exclude.txt" \
  "$test_root/snapshot/" "$test_root/target/"
[[ -f "$test_root/target/backups/database.dump" ]]
[[ -f "$test_root/target/future-runtime/state" ]]
[[ ! -e "$test_root/target/stale.txt" ]]

# What rsync-exclude.txt actually drops, not merely which strings it contains.
# A grep for 'assets/' passes just as happily for an unanchored rule that also
# eats frontend/src/assets -- rsync matches a slash-less pattern against the
# END of a path at every depth -- and the VPS build would then fail on files
# that are present locally. Likewise the station's icon must survive the sync
# (genwinres reads it during the image build) while its generated .syso must
# not (the build regenerates it, and a stale one would be linked instead).
exclude_root="$(mktemp -d)"
trap 'rm -rf -- "$test_root" "$exclude_root"' EXIT
mkdir -p "$exclude_root/src"/{assets,docs,frontend/src/assets,backend/station/build,backend/station/cmd/avtotest-station}
: >"$exclude_root/src/assets/brand-master.png"
: >"$exclude_root/src/station.log"
: >"$exclude_root/src/docs/plan.md"
: >"$exclude_root/src/frontend/src/assets/nested.png"
: >"$exclude_root/src/backend/station/build/drivergo.ico"
: >"$exclude_root/src/backend/station/VERSION"
: >"$exclude_root/src/backend/station/cmd/avtotest-station/rsrc_windows_386.syso"
mkdir -p "$exclude_root/dst"
rsync -a --exclude-from="$ROOT/deploy/rsync-exclude.txt" \
  "$exclude_root/src/" "$exclude_root/dst/"
[[ ! -e "$exclude_root/dst/assets" ]]
[[ ! -e "$exclude_root/dst/station.log" ]]
[[ ! -e "$exclude_root/dst/docs" ]]
[[ ! -e "$exclude_root/dst/backend/station/cmd/avtotest-station/rsrc_windows_386.syso" ]]
[[ -f "$exclude_root/dst/frontend/src/assets/nested.png" ]]
[[ -f "$exclude_root/dst/backend/station/build/drivergo.ico" ]]
[[ -f "$exclude_root/dst/backend/station/VERSION" ]]

printf 'sync-to-vps guards: ok\n'
