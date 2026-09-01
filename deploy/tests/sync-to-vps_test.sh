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

# Snapshot pruning is the only code in the script that deletes anything, so it
# is run for real against a temporary tree instead of being grepped for. The
# fixture deliberately mixes in entries that must survive at any age: a partial
# snapshot still being written, an unrelated directory, a loose file, and a
# symlink named exactly like the oldest snapshot. The symlink is the sharp case:
# its name matches the id pattern, so only the type checks keep it off the
# deletion list, and a future `rm -rf -- "$target"/` or `find -L` would reach
# through it into whatever it points at.
prune_root="$(mktemp -d)"
trap 'rm -rf -- "$test_root" "$exclude_root" "$prune_root"' EXIT
prune_dir="$prune_root/.deploy-rollbacks"
mkdir -p "$prune_dir"
for id in 20260810T073436Z-a0c51f61e20c \
          20260812T184509Z-fd049848ef6a \
          20260830T185846Z-66e8fcb22141 \
          20260831T101305Z-7607200ba63d \
          20260831T145332Z-22376a0dd34b; do
  mkdir -p "$prune_dir/$id/tree"
  : >"$prune_dir/$id/COMPLETE"
done
mkdir -p "$prune_dir/.partial-20260831T150000Z-abcdef123456"
mkdir -p "$prune_dir/not-a-snapshot"
: >"$prune_dir/loose-file"
symlink_bait="$(mktemp -d)"
: >"$symlink_bait/must-survive"
ln -s "$symlink_bait" "$prune_dir/20260101T000000Z-deadbeefcafe"

# A dry run reports but writes nothing.
prune_payload | bash -s -- "$prune_root" 2 dry-run >/dev/null
[[ -d "$prune_dir/20260810T073436Z-a0c51f61e20c" ]]
[[ "$(find "$prune_dir" -mindepth 1 -maxdepth 1 -type d | wc -l)" -eq 7 ]]

# --apply keeps the newest N by id and removes the oldest first.
prune_payload | bash -s -- "$prune_root" 2 apply >/dev/null
[[ ! -e "$prune_dir/20260810T073436Z-a0c51f61e20c" ]]
[[ ! -e "$prune_dir/20260812T184509Z-fd049848ef6a" ]]
[[ ! -e "$prune_dir/20260830T185846Z-66e8fcb22141" ]]
[[ -d "$prune_dir/20260831T101305Z-7607200ba63d/tree" ]]
[[ -d "$prune_dir/20260831T145332Z-22376a0dd34b/tree" ]]

# Everything that is not a well-formed snapshot directory is untouched.
[[ -d "$prune_dir/.partial-20260831T150000Z-abcdef123456" ]]
[[ -d "$prune_dir/not-a-snapshot" ]]
[[ -f "$prune_dir/loose-file" ]]
[[ -L "$prune_dir/20260101T000000Z-deadbeefcafe" ]]
[[ -f "$symlink_bait/must-survive" ]]
rm -rf -- "$symlink_bait"

# Re-running when already at the limit is a no-op, not an error.
prune_payload | bash -s -- "$prune_root" 2 apply >/dev/null
[[ -d "$prune_dir/20260831T101305Z-7607200ba63d" ]]

# A missing rollback root is tolerated; a finished deploy must not fail on it.
prune_payload | bash -s -- "$(mktemp -d)" 10 apply >/dev/null

# Nonsense retention values and modes are rejected before anything is deleted.
for bad in 0 -1 abc 1000 ''; do
  if prune_payload | bash -s -- "$prune_root" "$bad" apply >/dev/null 2>&1; then
    printf 'expected rejection of keep=%q\n' "$bad" >&2
    exit 1
  fi
done
if prune_payload | bash -s -- "$prune_root" 2 delete-everything >/dev/null 2>&1; then
  printf 'expected rejection of unknown prune mode\n' >&2
  exit 1
fi
[[ "$(find "$prune_dir" -mindepth 1 -maxdepth 1 -type d | wc -l)" -eq 4 ]]

printf 'sync-to-vps guards: ok\n'
