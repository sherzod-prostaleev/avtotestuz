#!/usr/bin/env bash
# Validate the local prerequisites for a provider-backed encrypted off-site
# backup. This script intentionally never reads or prints rclone credentials.
set -euo pipefail
umask 077

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SCRIPT_DIR/lib.sh"

AGE_RECIPIENT="${AGE_RECIPIENT:-}"
RCLONE_REMOTE="${RCLONE_REMOTE:-}"
RCLONE_CONFIG="${RCLONE_CONFIG:-/etc/drivergo/rclone.conf}"
OFFSITE_PREFLIGHT_SKIP_REMOTE="${OFFSITE_PREFLIGHT_SKIP_REMOTE:-0}"

[[ -n "${AGE_RECIPIENT//[[:space:]]/}" ]] ||
  backup_die "AGE_RECIPIENT must be set"
validate_remote_base "$RCLONE_REMOTE" >/dev/null 2>&1 ||
  backup_die "RCLONE_REMOTE is invalid"
require_bool "OFFSITE_PREFLIGHT_SKIP_REMOTE" "$OFFSITE_PREFLIGHT_SKIP_REMOTE"

[[ -f "$RCLONE_CONFIG" && ! -L "$RCLONE_CONFIG" ]] ||
  backup_die "rclone configuration must be a regular, non-symlink file"
config_mode="$(stat -c '%a' -- "$RCLONE_CONFIG" 2>/dev/null)" ||
  backup_die "cannot inspect rclone configuration permissions"
[[ "$config_mode" == "600" ]] ||
  backup_die "rclone configuration must have mode 0600"

if [[ "$OFFSITE_PREFLIGHT_SKIP_REMOTE" == "0" ]]; then
  require_command rclone
  rclone lsd "$RCLONE_REMOTE" >/dev/null 2>&1 ||
    backup_die "rclone remote reachability check failed"
fi

echo "off-site preflight passed"
