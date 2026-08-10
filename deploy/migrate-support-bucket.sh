#!/usr/bin/env bash
# Copy legacy media/support/* objects into the resolved authenticated support
# bucket. The existing MINIO_BUCKET contract is honored. Source and unrelated
# target objects are intentionally retained; no remove operation is used.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODE=dry-run

usage() {
  printf 'Usage: %s [--apply]\n' "$0"
  printf 'Default is read-only inventory. --apply copies but never deletes source objects.\n'
}

while (($#)); do
  case "$1" in
    --apply) MODE=apply ;;
    --dry-run) MODE=dry-run ;;
    -h|--help) usage; exit 0 ;;
    *) printf 'unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done

ENV_FILE="$ROOT/deploy/app.env"
[[ -f "$ENV_FILE" ]] || { echo "missing protected deploy/app.env" >&2; exit 2; }
[[ "$(stat -c '%a' "$ENV_FILE")" == "600" ]] || {
  echo "deploy/app.env must have mode 600" >&2; exit 2;
}

compose=(docker compose -f "$ROOT/deploy/docker-compose.prod.yml" --env-file "$ENV_FILE")
"${compose[@]}" config --quiet

read_only='set -eu
mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
target_bucket="${MINIO_SUPPORT_BUCKET:-${MINIO_BUCKET:-support-attachments}}"
case "$target_bucket" in ""|*[!a-z0-9.-]*) echo "invalid support bucket" >&2; exit 2;; esac
mc stat local/media >/dev/null
if [ "$target_bucket" = media ]; then
  echo "support target is media; prefix policy is private and no cross-bucket copy is needed"
  exit 0
fi
version_info="$(mc version info --json "local/$target_bucket")"
case "$version_info" in *'"status":"enabled"'*) ;; *) echo "target bucket versioning is not enabled" >&2; exit 7;; esac
echo "legacy source usage:"
mc du local/media/support 2>/dev/null || true
echo "private target usage:"
mc du "local/$target_bucket/support" 2>/dev/null || true
if mc ls local/media/support >/dev/null 2>&1; then
  echo "copy preview (immutable attachment keys; no target/source deletion):"
  mc mirror --dry-run --overwrite local/media/support "local/$target_bucket/support"
fi'

"${compose[@]}" run --rm --no-deps --entrypoint /bin/sh minio-init -c "$read_only"

if [[ "$MODE" != apply ]]; then
  echo "dry-run only: no objects copied; re-run with --apply after policy/API rollout"
  exit 0
fi

copy_only='set -eu
set -o pipefail
mc alias set local http://minio:9000 "$MINIO_ROOT_USER" "$MINIO_ROOT_PASSWORD" >/dev/null
target_bucket="${MINIO_SUPPORT_BUCKET:-${MINIO_BUCKET:-support-attachments}}"
case "$target_bucket" in ""|*[!a-z0-9.-]*) echo "invalid support bucket" >&2; exit 2;; esac
mc stat local/media >/dev/null
if [ "$target_bucket" = media ]; then
  echo "support target is media; prefix policy is private and no cross-bucket copy is needed"
  exit 0
fi
version_info="$(mc version info --json "local/$target_bucket")"
case "$version_info" in *'"status":"enabled"'*) ;; *) echo "target bucket versioning is not enabled" >&2; exit 7;; esac
if mc ls local/media/support >/dev/null 2>&1; then
  # Copy only missing immutable UUID keys. Existing target objects are never
  # overwritten, and source/target-only objects are never removed.
  object_list="/tmp/support-migration.$$"
  trap "rm -f -- \"$object_list\"" EXIT HUP INT TERM
  mc find local/media/support --print "{}" >"$object_list"
  copied=0
  verified_existing=0
  object_sha256() {
    digest="$(mc cat "$1" | sha256sum | cut -d " " -f 1)"
    case "$digest" in
      ""|*[!0-9a-f]*) echo "failed to hash object: $1" >&2; exit 4;;
    esac
    [ "${#digest}" -eq 64 ] || { echo "invalid SHA-256 length for $1" >&2; exit 4; }
    printf "%s\n" "$digest"
  }
  while IFS= read -r source; do
    [ -n "$source" ] || continue
    case "$source" in
      local/media/support/*) ;;
      *) echo "unexpected source key: $source" >&2; exit 3;;
    esac
    relative="${source#local/media/}"
    target="local/$target_bucket/$relative"
    source_digest="$(object_sha256 "$source")"
    stat_output=""
    if stat_output="$(mc stat --json "$target" 2>&1)"; then
      target_digest="$(object_sha256 "$target")"
      [ "$source_digest" = "$target_digest" ] || {
        echo "collision: existing target differs from legacy source: $relative" >&2
        exit 5
      }
      verified_existing=$((verified_existing + 1))
      continue
    fi
    # Only exact S3 not-found codes authorize a write. Permission, timeout, or
    # transport failures abort instead of being mistaken for a missing key.
    case "$stat_output" in
      *'"Code":"NoSuchKey"'*|*'"Code":"NoSuchObject"'*) ;;
      *) echo "target stat failed without an exact not-found response: $relative" >&2; exit 7;;
    esac
    mc cp "$source" "$target"
    target_digest="$(object_sha256 "$target")"
    [ "$source_digest" = "$target_digest" ] || {
      echo "copy verification failed: $relative" >&2
      exit 6
    }
    copied=$((copied + 1))
  done <"$object_list"
  rm -f "$object_list"
  trap - EXIT HUP INT TERM
  echo "copied=$copied verified_existing=$verified_existing"
else
  echo "no legacy support prefix; nothing to copy"
fi
echo "copy complete; legacy source intentionally retained for authenticated fallback"
mc du "local/$target_bucket/support" 2>/dev/null || true'

"${compose[@]}" run --rm --no-deps --entrypoint /bin/sh minio-init -c "$copy_only"
