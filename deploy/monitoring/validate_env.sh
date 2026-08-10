#!/usr/bin/env bash
# Validate monitoring image references without evaluating the env file as shell.
set -euo pipefail

ENV_FILE="${1:-}"
if [[ -z "$ENV_FILE" || ! -f "$ENV_FILE" || -L "$ENV_FILE" ]]; then
  printf 'usage: %s NON_SYMLINK_ENV_FILE\n' "$0" >&2
  exit 2
fi

read_exact_value() {
  local key="$1"
  local count value
  count="$(awk -F= -v key="$key" '$1 == key { count++ } END { print count + 0 }' "$ENV_FILE")"
  if [[ "$count" != "1" ]]; then
    printf 'monitoring env: %s must occur exactly once\n' "$key" >&2
    return 1
  fi
  value="$(awk -F= -v key="$key" '$1 == key { sub(/^[^=]*=/, ""); print; exit }' "$ENV_FILE")"
  printf '%s' "$value"
}

for key in PROMETHEUS_IMAGE BLACKBOX_EXPORTER_IMAGE NODE_EXPORTER_IMAGE ALERTMANAGER_IMAGE; do
  value="$(read_exact_value "$key")" || exit 1
  if [[ ! "$value" =~ ^[A-Za-z0-9][A-Za-z0-9._:/-]*@sha256:[0-9a-f]{64}$ ]]; then
    printf 'monitoring env: %s must be an immutable registry/repository@sha256 reference\n' "$key" >&2
    exit 1
  fi
done

for port_key in PROMETHEUS_HOST_PORT ALERTMANAGER_HOST_PORT; do
  port_count="$(awk -F= -v key="$port_key" '$1 == key { count++ } END { print count + 0 }' "$ENV_FILE")"
  if [[ "$port_count" -gt 1 ]]; then
    printf 'monitoring env: %s may occur at most once\n' "$port_key" >&2
    exit 1
  fi
  if [[ "$port_count" == "1" ]]; then
    port="$(awk -F= -v key="$port_key" '$1 == key { sub(/^[^=]*=/, ""); print; exit }' "$ENV_FILE")"
    if [[ ! "$port" =~ ^[0-9]+$ ]] || (( 10#$port < 1024 || 10#$port > 65535 )); then
      printf 'monitoring env: %s must be between 1024 and 65535\n' "$port_key" >&2
      exit 1
    fi
  fi
done

webhook_file="$(read_exact_value ALERT_WEBHOOK_URL_FILE)" || exit 1
webhook_gid="$(read_exact_value ALERT_WEBHOOK_GID)" || exit 1
if [[ ! "$webhook_gid" =~ ^[0-9]+$ ]] || (( 10#$webhook_gid < 1 || 10#$webhook_gid > 2147483647 )); then
  printf 'monitoring env: ALERT_WEBHOOK_GID must be a positive numeric group id\n' >&2
  exit 1
fi
if [[ "$webhook_file" != /* || "$webhook_file" == *'/../'* || "$webhook_file" == */.. || \
      "$webhook_file" == *'//'* || "$webhook_file" == "/" ]]; then
  printf 'monitoring env: ALERT_WEBHOOK_URL_FILE must be a safe absolute path\n' >&2
  exit 1
fi
if [[ ! -f "$webhook_file" || -L "$webhook_file" ]]; then
  printf 'monitoring env: ALERT_WEBHOOK_URL_FILE must be a regular non-symlink file\n' >&2
  exit 1
fi
webhook_mode="$(stat -c '%a' "$webhook_file")"
webhook_owner="$(stat -c '%u' "$webhook_file")"
webhook_group="$(stat -c '%g' "$webhook_file")"
if [[ "$webhook_mode" != "640" || "$webhook_owner" != "$EUID" || "$webhook_group" != "$webhook_gid" ]]; then
  printf 'monitoring env: ALERT_WEBHOOK_URL_FILE must be mode 640, owned by uid %s, and group %s\n' "$EUID" "$webhook_gid" >&2
  exit 1
fi
if [[ "$(wc -l <"$webhook_file" | tr -d ' ')" != "1" ]]; then
  printf 'monitoring env: webhook file must contain exactly one line\n' >&2
  exit 1
fi
webhook_url="$(<"$webhook_file")"
if [[ ! "$webhook_url" =~ ^https://[^[:space:]]+$ ]]; then
  printf 'monitoring env: webhook URL must use HTTPS and contain no whitespace\n' >&2
  exit 1
fi

printf 'monitoring env: immutable images and protected HTTPS receiver validated\n'
