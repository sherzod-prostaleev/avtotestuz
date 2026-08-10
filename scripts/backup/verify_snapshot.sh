#!/usr/bin/env bash
# Read-only verification of a complete Driver Go backup snapshot.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "$SCRIPT_DIR/lib.sh"

require_command realpath
require_command sha256sum

SNAPSHOT_DIR="${1:-}"
[[ -n "$SNAPSHOT_DIR" ]] || backup_die "usage: verify_snapshot.sh SNAPSHOT_DIR"
[[ -d "$SNAPSHOT_DIR" && ! -L "$SNAPSHOT_DIR" ]] || backup_die "snapshot directory missing or is a symlink: ${SNAPSHOT_DIR}"
SNAPSHOT_DIR="$(realpath -e "$SNAPSHOT_DIR")"
SNAPSHOT_NAME="$(basename "$SNAPSHOT_DIR")"
is_snapshot_name "$SNAPSHOT_NAME" || backup_die "invalid snapshot directory name: ${SNAPSHOT_NAME}"

MANIFEST="$SNAPSHOT_DIR/manifest.txt"
CHECKSUMS="$SNAPSHOT_DIR/SHA256SUMS"
[[ -f "$MANIFEST" && ! -L "$MANIFEST" ]] || backup_die "manifest.txt missing or unsafe"
[[ -f "$CHECKSUMS" && ! -L "$CHECKSUMS" ]] || backup_die "SHA256SUMS missing or unsafe"

required_manifest_keys=(
  format snapshot created_at backup_started_at capture_duration_seconds source_host git_commit encryption
  component_file_postgres component_file_redis component_file_minio component_file_humo
  postgres_database minio_consistency humo_consistency technical_backup_interval
)
for key in "${required_manifest_keys[@]}"; do
  occurrences="$(awk -F= -v wanted="$key" '$1 == wanted { count++ } END { print count + 0 }' "$MANIFEST")"
  [[ "$occurrences" == "1" ]] || backup_die "manifest key ${key} must occur exactly once"
done

[[ "$(manifest_value "$MANIFEST" format)" == "drivergo-full-backup-v1" ]] || backup_die "unsupported manifest format"
[[ "$(manifest_value "$MANIFEST" snapshot)" == "$SNAPSHOT_NAME" ]] || backup_die "manifest snapshot does not match directory"
CREATED_AT="$(manifest_value "$MANIFEST" created_at)"
[[ "$CREATED_AT" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]] || \
  backup_die "manifest created_at is not canonical UTC"
BACKUP_STARTED_AT="$(manifest_value "$MANIFEST" backup_started_at)"
[[ "$BACKUP_STARTED_AT" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]] || \
  backup_die "manifest backup_started_at is not canonical UTC"
CAPTURE_DURATION_SECONDS="$(manifest_value "$MANIFEST" capture_duration_seconds)"
[[ "$CAPTURE_DURATION_SECONDS" =~ ^[0-9]+$ ]] || backup_die "manifest capture duration is invalid"
ENCRYPTION="$(manifest_value "$MANIFEST" encryption)"
[[ "$ENCRYPTION" == "age" || "$ENCRYPTION" == "none" ]] || backup_die "unsupported encryption mode: ${ENCRYPTION}"
[[ "$(manifest_value "$MANIFEST" postgres_database)" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || \
  backup_die "manifest contains an unsafe PostgreSQL database name"
[[ "$(manifest_value "$MANIFEST" minio_consistency)" == "live-volume-tar-stream" ]] || \
  backup_die "manifest MinIO consistency mode mismatch"
[[ "$(manifest_value "$MANIFEST" humo_consistency)" == "sqlite-online-backup" ]] || \
  backup_die "manifest Humo consistency mode mismatch"

declare -a expected=(manifest.txt)
for key in postgres redis minio humo; do
  file="$(manifest_value "$MANIFEST" "component_file_${key}")"
  [[ -n "$file" && "$file" != */* && "$file" != .* ]] || backup_die "unsafe component filename for ${key}: ${file}"
  [[ -f "$SNAPSHOT_DIR/$file" && ! -L "$SNAPSHOT_DIR/$file" && -s "$SNAPSHOT_DIR/$file" ]] || \
    backup_die "component ${key} missing, empty, or unsafe: ${file}"
  if [[ "$ENCRYPTION" == "age" ]]; then
    [[ "$file" == *.age ]] || backup_die "encrypted snapshot contains plaintext-named component: ${file}"
  else
    [[ "$file" != *.age ]] || backup_die "plaintext manifest references age component: ${file}"
  fi
  size_key="size_${file//[^A-Za-z0-9]/_}"
  size_occurrences="$(awk -F= -v wanted="$size_key" '$1 == wanted { count++ } END { print count + 0 }' "$MANIFEST")"
  [[ "$size_occurrences" == "1" ]] || backup_die "manifest size key ${size_key} must occur exactly once"
  declared_size="$(manifest_value "$MANIFEST" "$size_key")"
  [[ "$declared_size" =~ ^[0-9]+$ ]] || backup_die "manifest size for ${file} is invalid"
  actual_size="$(wc -c <"$SNAPSHOT_DIR/$file" | tr -d ' ')"
  [[ "$actual_size" == "$declared_size" ]] || backup_die "manifest size mismatch for ${file}"
  expected+=("$file")
done

declare -A allowed=()
declare -A seen=()
for file in "${expected[@]}"; do
  allowed["$file"]=1
done
allowed[SHA256SUMS]=1
if [[ -e "$SNAPSHOT_DIR/REMOTE_COMPLETE" || -L "$SNAPSHOT_DIR/REMOTE_COMPLETE" ]]; then
  marker="$SNAPSHOT_DIR/REMOTE_COMPLETE"
  [[ -f "$marker" && ! -L "$marker" ]] || backup_die "REMOTE_COMPLETE is unsafe"
  [[ "$(wc -l <"$marker" | tr -d ' ')" == "2" ]] || backup_die "REMOTE_COMPLETE must contain exactly two lines"
  for marker_key in snapshot sha256sums_sha256; do
    marker_occurrences="$(awk -F= -v wanted="$marker_key" '$1 == wanted { count++ } END { print count + 0 }' "$marker")"
    [[ "$marker_occurrences" == "1" ]] || backup_die "REMOTE_COMPLETE key ${marker_key} must occur exactly once"
  done
  marker_snapshot="$(manifest_value "$marker" snapshot)"
  marker_digest="$(manifest_value "$marker" sha256sums_sha256)"
  [[ "$marker_snapshot" == "$SNAPSHOT_NAME" ]] || backup_die "REMOTE_COMPLETE snapshot mismatch"
  [[ "$marker_digest" =~ ^[0-9a-fA-F]{64}$ ]] || backup_die "REMOTE_COMPLETE digest is invalid"
  actual_checksums_digest="$(sha256sum -- "$CHECKSUMS" | awk '{ print $1 }')"
  [[ "${actual_checksums_digest,,}" == "${marker_digest,,}" ]] || backup_die "REMOTE_COMPLETE digest mismatch"
  allowed[REMOTE_COMPLETE]=1
fi

shopt -s nullglob dotglob
entries=("$SNAPSHOT_DIR"/*)
shopt -u nullglob dotglob
for entry in "${entries[@]}"; do
  file="$(basename "$entry")"
  [[ -n "${allowed[$file]:-}" ]] || backup_die "snapshot contains unexpected entry: ${file}"
  [[ -f "$entry" && ! -L "$entry" ]] || backup_die "snapshot entry is not a regular non-symlink file: ${file}"
done
[[ "${#entries[@]}" -eq "${#allowed[@]}" ]] || backup_die "snapshot entry count mismatch"

checksum_count=0
while IFS= read -r line; do
  [[ "$line" =~ ^[0-9a-fA-F]{64}[[:space:]][[:space:]]([^/]+)$ ]] || backup_die "malformed SHA256SUMS line"
  file="${BASH_REMATCH[1]}"
  [[ -n "${allowed[$file]:-}" ]] || backup_die "SHA256SUMS references unexpected path: ${file}"
  [[ -z "${seen[$file]:-}" ]] || backup_die "SHA256SUMS contains duplicate entry: ${file}"
  seen["$file"]=1
  checksum_count="$((checksum_count + 1))"
done <"$CHECKSUMS"
[[ "$checksum_count" -eq "${#expected[@]}" ]] || backup_die "SHA256SUMS entry count mismatch"
for file in "${expected[@]}"; do
  [[ -n "${seen[$file]:-}" ]] || backup_die "SHA256SUMS is missing expected entry: ${file}"
done

(
  cd "$SNAPSHOT_DIR"
  sha256sum --check --strict SHA256SUMS
)

echo "snapshot verification: ok (${SNAPSHOT_NAME}, encryption=${ENCRYPTION})"
