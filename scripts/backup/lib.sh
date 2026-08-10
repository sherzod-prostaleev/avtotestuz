#!/usr/bin/env bash
# Shared helpers for Driver Go backup and restore-drill scripts.
# shellcheck shell=bash

backup_die() {
  echo "backup: $*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || backup_die "required command not found: $1"
}

require_uint() {
  local name="$1"
  local value="$2"
  [[ "$value" =~ ^(0|[1-9][0-9]*)$ ]] || \
    backup_die "${name} must be a canonical unsigned integer (got ${value})"
}

require_positive_uint() {
  local name="$1"
  local value="$2"
  require_uint "$name" "$value"
  (( 10#$value > 0 )) || backup_die "${name} must be greater than zero"
}

require_bool() {
  local name="$1"
  local value="$2"
  [[ "$value" == "0" || "$value" == "1" ]] || \
    backup_die "${name} must be 0 or 1 (got ${value})"
}

repo_root() {
  cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd
}

# Avoid a shell-evaluated COMPOSE string in the production path. Operators may
# point at a different compose file/env file, but each remains one quoted argv.
compose_cmd() {
  local -a args=(docker compose)
  if [[ -n "${COMPOSE_FILE:-}" ]]; then
    args+=(-f "$COMPOSE_FILE")
  fi
  if [[ -n "${COMPOSE_EXTRA_FILE:-}" ]]; then
    args+=(-f "$COMPOSE_EXTRA_FILE")
  fi
  if [[ -n "${COMPOSE_ENV_FILE:-}" ]]; then
    args+=(--env-file "$COMPOSE_ENV_FILE")
  fi
  "${args[@]}" "$@"
}

compose_container_id() {
  local service="$1"
  local -a ids=()
  mapfile -t ids < <(compose_cmd ps -q "$service")
  [[ "${#ids[@]}" -eq 1 && -n "${ids[0]}" ]] || \
    backup_die "service ${service} must have exactly one running container"
  printf '%s\n' "${ids[0]}"
}

validate_service_name() {
  local name="$1"
  [[ "$name" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]] || \
    backup_die "unsafe Compose service name: ${name}"
}

is_snapshot_name() {
  [[ "$1" =~ ^drivergo-[0-9]{8}T[0-9]{6}Z$ ]]
}

is_partial_snapshot_name() {
  [[ "$1" =~ ^\.partial-drivergo-[0-9]{8}T[0-9]{6}Z$ ]]
}

validate_backup_root() {
  local root="$1"
  local relative mode owner
  [[ "$root" = /* ]] || backup_die "BACKUP_ROOT must be absolute: ${root}"
  [[ "$root" != "/" ]] || backup_die "BACKUP_ROOT must not be /"
  relative="${root#/}"
  [[ "$relative" == */* ]] || backup_die "BACKUP_ROOT is too broad: ${root}"
  if [[ -e "$root" ]]; then
    [[ -d "$root" && ! -L "$root" ]] || backup_die "BACKUP_ROOT must be a non-symlink directory: ${root}"
  else
    mkdir -p -m 0700 "$root"
  fi
  mode="$(stat -c '%a' "$root")"
  owner="$(stat -c '%u' "$root")"
  [[ "$owner" == "$EUID" ]] || backup_die "BACKUP_ROOT must be owned by uid ${EUID}: ${root}"
  (( 10#$mode % 100 == 0 )) || backup_die "BACKUP_ROOT must not be accessible by group/other (mode ${mode})"
}

validate_quarantine_root() {
  local backup_root="$1"
  local quarantine_root="$2"
  local backup_parent quarantine_parent mode owner
  [[ "$quarantine_root" = /* && "$(basename "${quarantine_root%/}")" == "quarantine" ]] || \
    backup_die "BACKUP_QUARANTINE_ROOT must be an absolute path ending in quarantine"
  backup_parent="$(realpath -e "$(dirname "$backup_root")")"
  quarantine_parent="$(realpath -e "$(dirname "$quarantine_root")")"
  [[ "$quarantine_parent" == "$backup_parent" ]] || \
    backup_die "BACKUP_QUARANTINE_ROOT must be a sibling of BACKUP_ROOT"
  if [[ -e "$quarantine_root" ]]; then
    [[ -d "$quarantine_root" && ! -L "$quarantine_root" ]] || \
      backup_die "BACKUP_QUARANTINE_ROOT must be a non-symlink directory"
  else
    mkdir -m 0700 -- "$quarantine_root"
  fi
  mode="$(stat -c '%a' "$quarantine_root")"
  owner="$(stat -c '%u' "$quarantine_root")"
  [[ "$owner" == "$EUID" ]] || backup_die "BACKUP_QUARANTINE_ROOT must be owned by uid ${EUID}"
  (( 10#$mode % 100 == 0 )) || \
    backup_die "BACKUP_QUARANTINE_ROOT must not be accessible by group/other"
}

# Restore-drill scripts may create and remove children only below a directory
# whose basename is exactly drivergo-restore-drill. This deliberately rejects
# broad paths such as /tmp, /var/tmp, /opt, or /.
validate_drill_root() {
  local root="$1"
  local mode owner
  [[ "$root" = /* ]] || backup_die "DRILL_ROOT must be absolute: ${root}"
  [[ "${root%/}" != "/" ]] || backup_die "DRILL_ROOT must not be /"
  [[ "$(basename "${root%/}")" == "drivergo-restore-drill" ]] || \
    backup_die "DRILL_ROOT basename must be drivergo-restore-drill: ${root}"
  if [[ -e "$root" ]]; then
    [[ -d "$root" && ! -L "$root" ]] || backup_die "DRILL_ROOT must be a non-symlink directory: ${root}"
  else
    mkdir -p -m 0700 "$root"
  fi
  mode="$(stat -c '%a' "$root")"
  owner="$(stat -c '%u' "$root")"
  [[ "$owner" == "$EUID" ]] || backup_die "DRILL_ROOT must be owned by uid ${EUID}: ${root}"
  (( 10#$mode % 100 == 0 )) || backup_die "DRILL_ROOT must not be accessible by group/other (mode ${mode})"
}

validate_scratch_child() {
  local root="$1"
  local child="$2"
  local root_real child_parent_real
  root_real="$(realpath -e "$root")"
  child_parent_real="$(realpath -e "$(dirname "$child")")"
  [[ "$child_parent_real" == "$root_real" || "$child_parent_real" == "$root_real"/* ]] || \
    backup_die "scratch path escapes DRILL_ROOT: ${child}"
  [[ "$(basename "$child")" =~ ^drivergo-drill\.[A-Za-z0-9._-]+$ ]] || \
    backup_die "scratch child has unsafe name: ${child}"
}

validate_remote_base() {
  local remote="$1"
  local remote_name path
  [[ "$remote" == *:* ]] || backup_die "RCLONE_REMOTE must use remote:path syntax"
  remote_name="${remote%%:*}"
  [[ "$remote_name" =~ ^[A-Za-z0-9._-]+$ ]] || backup_die "RCLONE_REMOTE contains an unsafe remote name"
  path="${remote#*:}"
  path="${path#/}"
  [[ -n "$path" && "$path" != "." && "$path" != "/" ]] || \
    backup_die "RCLONE_REMOTE must name a dedicated non-root prefix"
  [[ "$path" =~ ^[A-Za-z0-9._/-]+$ ]] || backup_die "RCLONE_REMOTE contains unsafe path characters"
  case "/$path/" in
    */../*|*/./*|*//*) backup_die "RCLONE_REMOTE contains an unsafe path segment" ;;
  esac
}

validate_lock_file() {
  local lock_file="$1"
  local parent mode owner
  [[ "$lock_file" = /* && "$lock_file" == *.lock ]] || \
    backup_die "lock file must be an absolute .lock path: ${lock_file}"
  parent="$(dirname "$lock_file")"
  if [[ -e "$parent" ]]; then
    [[ -d "$parent" && ! -L "$parent" ]] || backup_die "lock parent is not a safe directory: ${parent}"
  else
    mkdir -p -m 0700 "$parent"
  fi
  mode="$(stat -c '%a' "$parent")"
  owner="$(stat -c '%u' "$parent")"
  [[ "$owner" == "$EUID" ]] || backup_die "lock parent must be owned by uid ${EUID}: ${parent}"
  (( 10#$mode % 100 == 0 )) || backup_die "lock parent must not be accessible by group/other (mode ${mode})"
  [[ ! -L "$lock_file" ]] || backup_die "lock file must not be a symlink: ${lock_file}"
  if [[ -e "$lock_file" ]]; then
    [[ -f "$lock_file" ]] || backup_die "lock path must be a regular file: ${lock_file}"
    owner="$(stat -c '%u' "$lock_file")"
    [[ "$owner" == "$EUID" ]] || backup_die "lock file must be owned by uid ${EUID}: ${lock_file}"
  fi
}

manifest_value() {
  local manifest="$1"
  local key="$2"
  awk -F= -v wanted="$key" '$1 == wanted { sub(/^[^=]*=/, ""); print; exit }' "$manifest"
}

safe_remove_scratch() {
  local root="$1"
  local child="$2"
  [[ -n "$child" && -d "$child" ]] || return 0
  validate_scratch_child "$root" "$child"
  rm -rf -- "$child"
}

validate_runtime_drill_image() {
  local kind="$1"
  local image="$2"
  local pattern
  case "$kind" in
    redis)
      pattern='^(redis|docker\.io/library/redis)(:[A-Za-z0-9][A-Za-z0-9._-]{0,127})?@sha256:[0-9a-f]{64}$'
      ;;
    minio)
      pattern='^(minio/minio|docker\.io/minio/minio)(:[A-Za-z0-9][A-Za-z0-9._-]{0,127})?@sha256:[0-9a-f]{64}$'
      ;;
    minio-mc)
      pattern='^(minio/mc|docker\.io/minio/mc)(:[A-Za-z0-9][A-Za-z0-9._-]{0,127})?@sha256:[0-9a-f]{64}$'
      ;;
    *) backup_die "unknown runtime drill image kind: ${kind}" ;;
  esac
  [[ -n "$image" && "$image" =~ $pattern ]] || \
    backup_die "${kind} runtime drill image must use its allowlisted upstream repository and an immutable sha256 digest"
}

is_runtime_drill_container_name() {
  local kind="$1"
  local name="$2"
  case "$kind" in
    redis) [[ "$name" =~ ^drivergo-redis-drill-[a-z0-9]{6,32}$ ]] ;;
    minio-server) [[ "$name" =~ ^drivergo-minio-server-drill-[a-z0-9]{6,32}$ ]] ;;
    minio-client) [[ "$name" =~ ^drivergo-minio-client-drill-[a-z0-9]{6,32}$ ]] ;;
    *) return 1 ;;
  esac
}

validate_runtime_drill_container_name() {
  local kind="$1"
  local name="$2"
  is_runtime_drill_container_name "$kind" "$name" || \
    backup_die "unsafe ${kind} runtime drill container name: ${name}"
}

is_runtime_drill_network_name() {
  [[ "$1" =~ ^drivergo-minio-drill-net-[a-z0-9]{6,32}$ ]]
}

validate_runtime_drill_network_name() {
  is_runtime_drill_network_name "$1" || \
    backup_die "unsafe runtime drill network name: $1"
}

validate_runtime_object_id() {
  local kind="$1"
  local id="$2"
  [[ "$id" =~ ^[0-9a-f]{12,64}$ ]] || \
    backup_die "unsafe ${kind} runtime object id returned by Docker"
}

runtime_drill_token() {
  local scratch="$1"
  local token
  token="${scratch##*.}"
  [[ "$token" =~ ^[A-Za-z0-9]{6}$ ]] || \
    backup_die "cannot derive a safe runtime drill token from scratch path"
  printf '%s\n' "${token,,}"
}

validate_runtime_bind_source() {
  local drill_root="$1"
  local source="$2"
  local expected_type="$3"
  local root_real source_real
  root_real="$(realpath -e "$drill_root")"
  source_real="$(realpath -e "$source")"
  case "$source_real" in
    "$root_real"/*) ;;
    *) backup_die "runtime bind source escapes DRILL_ROOT: ${source}" ;;
  esac
  [[ "$source_real" != *','* && "$source_real" != *$'\n'* && "$source_real" != *$'\r'* ]] || \
    backup_die "runtime bind source contains an unsafe Docker mount delimiter"
  case "$expected_type" in
    file) [[ -f "$source_real" && ! -L "$source" ]] || backup_die "runtime bind source is not a safe file" ;;
    directory) [[ -d "$source_real" && ! -L "$source" ]] || backup_die "runtime bind source is not a safe directory" ;;
    *) backup_die "unknown runtime bind source type: ${expected_type}" ;;
  esac
}

safe_remove_runtime_container() {
  local kind="$1"
  local name="$2"
  local id="$3"
  local identity
  is_runtime_drill_container_name "$kind" "$name" || {
    echo "backup: refusing cleanup of unsafe runtime container name: ${name}" >&2
    return 1
  }
  [[ "$id" =~ ^[0-9a-f]{12,64}$ ]] || {
    echo "backup: refusing cleanup of unsafe runtime container id" >&2
    return 1
  }
  if ! identity="$(docker inspect --format '{{.Name}}|{{index .Config.Labels "com.drivergo.restore-drill"}}' "$id" 2>/dev/null)"; then
    echo "backup: refusing cleanup because runtime container identity cannot be verified" >&2
    return 1
  fi
  [[ "$identity" == "/${name}|${kind}" ]] || {
    echo "backup: refusing cleanup of runtime container whose name/label identity changed" >&2
    return 1
  }
  docker rm -f -- "$id" >/dev/null
}

safe_remove_runtime_network() {
  local name="$1"
  local id="$2"
  local identity
  is_runtime_drill_network_name "$name" || {
    echo "backup: refusing cleanup of unsafe runtime network name: ${name}" >&2
    return 1
  }
  [[ "$id" =~ ^[0-9a-f]{12,64}$ ]] || {
    echo "backup: refusing cleanup of unsafe runtime network id" >&2
    return 1
  }
  if ! identity="$(docker network inspect --format '{{.Name}}|{{index .Labels "com.drivergo.restore-drill"}}' "$id" 2>/dev/null)"; then
    echo "backup: refusing cleanup because runtime network identity cannot be verified" >&2
    return 1
  fi
  [[ "$identity" == "${name}|minio" ]] || {
    echo "backup: refusing cleanup of runtime network whose name/label identity changed" >&2
    return 1
  }
  docker network rm "$id" >/dev/null
}
