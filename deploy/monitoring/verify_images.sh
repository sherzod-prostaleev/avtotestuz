#!/usr/bin/env bash
# Fail closed unless the exact deployment images have no HIGH/CRITICAL findings.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${1:-}"
TRIVY_IMAGE="${TRIVY_IMAGE:-aquasec/trivy@sha256:7cced7cae583819fc7806d4cbc0dbbc7cad18b99f7d3e235192e6da8c091045c}"
TARGET_PLATFORM="${TARGET_PLATFORM:-linux/amd64}"

if [[ -z "$ENV_FILE" ]]; then
  printf 'usage: %s MONITORING_ENV_FILE\n' "$0" >&2
  exit 2
fi

command -v docker >/dev/null 2>&1 || {
  printf 'monitoring image verification: docker is required\n' >&2
  exit 1
}

"$SCRIPT_DIR/validate_env.sh" "$ENV_FILE"

read_exact_value() {
  local key="$1" count value
  count="$(awk -F= -v key="$key" '$1 == key { count++ } END { print count + 0 }' "$ENV_FILE")"
  [[ "$count" == "1" ]] || return 1
  value="$(awk -F= -v key="$key" '$1 == key { sub(/^[^=]*=/, ""); print; exit }' "$ENV_FILE")"
  printf '%s' "$value"
}

for key in PROMETHEUS_IMAGE BLACKBOX_EXPORTER_IMAGE NODE_EXPORTER_IMAGE ALERTMANAGER_IMAGE; do
  image="$(read_exact_value "$key")" || {
    printf 'monitoring image verification: cannot read %s\n' "$key" >&2
    exit 1
  }

  docker pull "$image" >/dev/null
  platform="$(docker image inspect "$image" --format '{{.Os}}/{{.Architecture}}')"
  if [[ "$platform" != "$TARGET_PLATFORM" ]]; then
    printf 'monitoring image verification: %s resolved to %s, expected %s\n' \
      "$image" "$platform" "$TARGET_PLATFORM" >&2
    exit 1
  fi

  printf 'monitoring image verification: scanning %s (%s)\n' "$image" "$platform"
  docker image save "$image" | docker run --rm -i "$TRIVY_IMAGE" image \
    --input - \
    --scanners vuln \
    --severity HIGH,CRITICAL \
    --ignore-unfixed=false \
    --exit-code 1
done

printf 'monitoring image verification: all exact images have zero HIGH/CRITICAL findings\n'
