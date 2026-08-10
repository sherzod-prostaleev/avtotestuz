#!/usr/bin/env bash
# Publish bounded, allowlisted host/backup/container metrics for node_exporter.
# This runs as a short-lived systemd oneshot; no long-lived container receives
# access to the Docker socket or host filesystem.
set -uo pipefail

umask 022

TEXTFILE_DIR="${TEXTFILE_DIR:-/var/lib/drivergo-monitoring/textfile}"
OUTPUT_FILE="${TEXTFILE_DIR}/drivergo_ops.prom"
LOCK_FILE="${MONITORING_LOCK_FILE:-/run/drivergo-monitoring/collector.lock}"
COMPOSE_FILE="${COMPOSE_FILE:-/opt/drivergo/deploy/docker-compose.prod.yml}"
COMPOSE_ENV_FILE="${COMPOSE_ENV_FILE:-/opt/drivergo/deploy/app.env}"
BACKUP_ROOT="${BACKUP_ROOT:-/var/backups/drivergo/full}"
BACKUP_VERIFY_SCRIPT="${BACKUP_VERIFY_SCRIPT:-/opt/drivergo/scripts/backup/verify_snapshot.sh}"
BACKUP_VERIFY_CACHE_DIR="${BACKUP_VERIFY_CACHE_DIR:-${TEXTFILE_DIR%/textfile}/backup-verify-cache}"
BACKUP_VERIFY_CACHE_TTL_SECONDS="${BACKUP_VERIFY_CACHE_TTL_SECONDS:-90000}"
TLS_CERT_FILE="${TLS_CERT_FILE:-}"
BACKUP_SERVICE_UNIT="drivergo-backup.service"
BACKUP_TIMER_UNIT="drivergo-backup.timer"

METRICS_FILE=""
COLLECTOR_SUCCESS=1
OUTPUT_SUCCESS=1

log_error() {
  printf 'drivergo-monitoring: %s\n' "$*" >&2
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || {
    log_error "required command not found: $1"
    return 1
  }
}

validate_absolute_file_setting() {
  local name="$1"
  local value="$2"
  [[ "$value" = /* ]] || {
    log_error "${name} must be an absolute path"
    return 1
  }
}

validate_verifier() {
  local mode owner
  validate_absolute_file_setting BACKUP_VERIFY_SCRIPT "$BACKUP_VERIFY_SCRIPT" || return 1
  [[ -f "$BACKUP_VERIFY_SCRIPT" && ! -L "$BACKUP_VERIFY_SCRIPT" && -x "$BACKUP_VERIFY_SCRIPT" ]] || {
    log_error "BACKUP_VERIFY_SCRIPT must be an executable regular non-symlink file"
    return 1
  }
  owner="$(stat -c '%u' "$BACKUP_VERIFY_SCRIPT")" || return 1
  mode="$(stat -c '%a' "$BACKUP_VERIFY_SCRIPT")" || return 1
  [[ "$owner" == "$EUID" ]] || {
    log_error "BACKUP_VERIFY_SCRIPT must be owned by uid ${EUID}"
    return 1
  }
  (( (8#$mode & 0022) == 0 )) || {
    log_error "BACKUP_VERIFY_SCRIPT must not be group/other writable"
    return 1
  }
}

validate_verify_cache() {
  local mode owner
  [[ "$BACKUP_VERIFY_CACHE_DIR" = /* && "$(basename "$BACKUP_VERIFY_CACHE_DIR")" == "backup-verify-cache" ]] || {
    log_error "BACKUP_VERIFY_CACHE_DIR must be absolute with basename backup-verify-cache"
    return 1
  }
  [[ "$BACKUP_VERIFY_CACHE_TTL_SECONDS" =~ ^[0-9]+$ ]] || {
    log_error "BACKUP_VERIFY_CACHE_TTL_SECONDS must be numeric"
    return 1
  }
  (( 10#$BACKUP_VERIFY_CACHE_TTL_SECONDS >= 3600 && 10#$BACKUP_VERIFY_CACHE_TTL_SECONDS <= 604800 )) || {
    log_error "BACKUP_VERIFY_CACHE_TTL_SECONDS must be 3600..604800"
    return 1
  }
  if [[ -e "$BACKUP_VERIFY_CACHE_DIR" ]]; then
    [[ -d "$BACKUP_VERIFY_CACHE_DIR" && ! -L "$BACKUP_VERIFY_CACHE_DIR" ]] || return 1
  else
    mkdir -p -m 0750 -- "$BACKUP_VERIFY_CACHE_DIR" || return 1
  fi
  owner="$(stat -c '%u' "$BACKUP_VERIFY_CACHE_DIR")" || return 1
  mode="$(stat -c '%a' "$BACKUP_VERIFY_CACHE_DIR")" || return 1
  [[ "$owner" == "$EUID" ]] || return 1
  (( (8#$mode & 0022) == 0 )) || return 1
}

snapshot_fingerprint() {
  local directory="$1" entry
  local -a entries=()
  [[ -f "$directory/manifest.txt" && ! -L "$directory/manifest.txt" && \
     -f "$directory/SHA256SUMS" && ! -L "$directory/SHA256SUMS" ]] || return 1
  shopt -s nullglob dotglob
  entries=("$directory"/*)
  shopt -u nullglob dotglob
  {
    sha256sum "$directory/manifest.txt" "$directory/SHA256SUMS" || return 1
    stat -c '%d:%i:%s:%Y:%Z:%f:%n' "$directory" || return 1
    for entry in "${entries[@]}"; do
      [[ ! -L "$entry" && -f "$entry" ]] || return 1
      stat -c '%d:%i:%s:%Y:%Z:%f:%n' "$entry" || return 1
    done
  } | sha256sum | awk '{print $1}'
}

verify_snapshot_cached() {
  local directory="$1" base="$2" fingerprint cache now cached_fingerprint verified_at status tmp age
  fingerprint="$(snapshot_fingerprint "$directory")" || return 1
  cache="$BACKUP_VERIFY_CACHE_DIR/$base.cache"
  now="$(date -u +%s)" || return 1
  if [[ -f "$cache" && ! -L "$cache" ]]; then
    cached_fingerprint="$(awk -F= '$1 == "fingerprint" {print $2; exit}' "$cache")"
    verified_at="$(awk -F= '$1 == "verified_at" {print $2; exit}' "$cache")"
    status="$(awk -F= '$1 == "status" {print $2; exit}' "$cache")"
    if [[ "$cached_fingerprint" == "$fingerprint" && "$verified_at" =~ ^[0-9]+$ && \
          ( "$status" == ok || "$status" == invalid ) ]]; then
      age="$((now - verified_at))"
      if (( age >= 0 && age <= BACKUP_VERIFY_CACHE_TTL_SECONDS )); then
        [[ "$status" == ok ]]
        return
      fi
    fi
  fi

  status=invalid
  if "$BACKUP_VERIFY_SCRIPT" "$directory" >/dev/null 2>&1; then
    status=ok
  fi
  tmp="$(mktemp "$BACKUP_VERIFY_CACHE_DIR/.${base}.cache.XXXXXX")" || return 1
  {
    printf 'fingerprint=%s\n' "$fingerprint"
    printf 'verified_at=%s\n' "$now"
    printf 'status=%s\n' "$status"
  } >"$tmp" || { rm -f -- "$tmp"; return 1; }
  chmod 0600 "$tmp" || { rm -f -- "$tmp"; return 1; }
  mv -f -- "$tmp" "$cache" || { rm -f -- "$tmp"; return 1; }
  [[ "$status" == ok ]]
}

validate_output_paths() {
  local lock_parent mode owner
  [[ "$TEXTFILE_DIR" = /* && "$(basename "$TEXTFILE_DIR")" == "textfile" ]] || {
    log_error "TEXTFILE_DIR must be an absolute path with basename textfile"
    return 1
  }
  [[ "$TEXTFILE_DIR" != "/" ]] || return 1
  if [[ -e "$TEXTFILE_DIR" ]]; then
    [[ -d "$TEXTFILE_DIR" && ! -L "$TEXTFILE_DIR" ]] || {
      log_error "TEXTFILE_DIR must be a non-symlink directory"
      return 1
    }
  else
    mkdir -p -m 0755 -- "$TEXTFILE_DIR" || return 1
  fi
  owner="$(stat -c '%u' "$TEXTFILE_DIR")"
  mode="$(stat -c '%a' "$TEXTFILE_DIR")"
  [[ "$owner" == "$EUID" ]] || {
    log_error "TEXTFILE_DIR must be owned by uid ${EUID}"
    return 1
  }
  (( (8#$mode & 0022) == 0 )) || {
    log_error "TEXTFILE_DIR must not be group/other writable"
    return 1
  }
  [[ ! -L "$OUTPUT_FILE" ]] || {
    log_error "output path must not be a symlink"
    return 1
  }
  if [[ -e "$OUTPUT_FILE" ]]; then
    [[ -f "$OUTPUT_FILE" && "$(stat -c '%u' "$OUTPUT_FILE")" == "$EUID" ]] || {
      log_error "existing output must be a regular file owned by uid ${EUID}"
      return 1
    }
  fi

  [[ "$LOCK_FILE" = /* && "$LOCK_FILE" == *.lock ]] || {
    log_error "MONITORING_LOCK_FILE must be an absolute .lock path"
    return 1
  }
  lock_parent="$(dirname "$LOCK_FILE")"
  [[ "$lock_parent" != "/" ]] || return 1
  if [[ -e "$lock_parent" ]]; then
    [[ -d "$lock_parent" && ! -L "$lock_parent" ]] || {
      log_error "lock parent must be a non-symlink directory"
      return 1
    }
  else
    mkdir -p -m 0700 -- "$lock_parent" || return 1
  fi
  owner="$(stat -c '%u' "$lock_parent")"
  mode="$(stat -c '%a' "$lock_parent")"
  [[ "$owner" == "$EUID" ]] || {
    log_error "lock parent must be owned by uid ${EUID}"
    return 1
  }
  (( (8#$mode & 0022) == 0 )) || {
    log_error "lock parent must not be group/other writable"
    return 1
  }
  [[ ! -L "$LOCK_FILE" ]] || {
    log_error "lock path must not be a symlink"
    return 1
  }
  if [[ -e "$LOCK_FILE" ]]; then
    [[ -f "$LOCK_FILE" && "$(stat -c '%u' "$LOCK_FILE")" == "$EUID" ]] || {
      log_error "existing lock must be a regular file owned by uid ${EUID}"
      return 1
    }
  fi
}

emit_header() {
  local name="$1"
  local help="$2"
  local type="$3"
  [[ "$OUTPUT_SUCCESS" == "1" ]] || return 0
  if ! printf '# HELP %s %s\n# TYPE %s %s\n' "$name" "$help" "$name" "$type" >>"$METRICS_FILE"; then
    OUTPUT_SUCCESS=0
    log_error "cannot write temporary metrics output"
  fi
}

emit_metric() {
  [[ "$OUTPUT_SUCCESS" == "1" ]] || return 0
  if ! printf '%s\n' "$1" >>"$METRICS_FILE"; then
    OUTPUT_SUCCESS=0
    log_error "cannot write temporary metrics output"
  fi
}

collect_containers() {
  local -a services=(postgres redis minio api humo-watcher web)
  local -a compose=(docker compose -f "$COMPOSE_FILE" --env-file "$COMPOSE_ENV_FILE")
  local service id details status health restarts extra
  local running healthy source_ok=1

  emit_header drivergo_container_running "Whether the allowlisted Compose service has one running container." gauge
  emit_header drivergo_container_healthy "Whether the allowlisted Compose service reports a healthy Docker health check." gauge
  emit_header drivergo_container_restarts_total "Docker restart count for the allowlisted Compose service container." counter

  for service in "${services[@]}"; do
    running=0
    healthy=0
    restarts=0
    id=""
    if ! id="$("${compose[@]}" ps -a -q "$service" 2>/dev/null)"; then
      log_error "cannot query Compose service ${service}"
      source_ok=0
    elif [[ -n "$id" && "$id" != *$'\n'* && "$id" =~ ^[A-Za-z0-9_.-]+$ ]]; then
      details=""
      if details="$(docker inspect --format '{{.State.Status}}|{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}|{{.RestartCount}}' "$id" 2>/dev/null)"; then
        IFS='|' read -r status health restarts extra <<<"$details"
        if [[ -n "$extra" || ! "$restarts" =~ ^[0-9]+$ ]]; then
          log_error "unexpected inspect output for ${service}"
          source_ok=0
          restarts=0
        else
          [[ "$status" == "running" ]] && running=1
          [[ "$health" == "healthy" ]] && healthy=1
        fi
      else
        log_error "cannot inspect Compose service ${service}"
        source_ok=0
      fi
    elif [[ -n "$id" ]]; then
      log_error "Compose service ${service} did not resolve to exactly one safe container id"
    fi
    emit_metric "drivergo_container_running{service=\"${service}\"} ${running}"
    emit_metric "drivergo_container_healthy{service=\"${service}\"} ${healthy}"
    emit_metric "drivergo_container_restarts_total{service=\"${service}\"} ${restarts}"
  done

  [[ "$source_ok" == "1" ]]
}

collect_backup_unit() {
  local service_result timer_state attempted_at attempted_epoch=0
  local source_ok=1 attempt_success=0 timer_active=0

  emit_header drivergo_backup_timer_active "Whether drivergo-backup.timer is active." gauge
  emit_header drivergo_backup_last_attempt_success "Whether the most recent backup service result was success." gauge
  emit_header drivergo_backup_last_attempt_unixtime "Unix time when the most recent backup service attempt finished." gauge

  service_result=""
  if service_result="$(systemctl show "$BACKUP_SERVICE_UNIT" --property=Result --value 2>/dev/null)"; then
    [[ "$service_result" == "success" ]] && attempt_success=1
  else
    log_error "cannot read ${BACKUP_SERVICE_UNIT} result"
    source_ok=0
  fi

  attempted_at=""
  if attempted_at="$(systemctl show "$BACKUP_SERVICE_UNIT" --property=ExecMainExitTimestamp --value 2>/dev/null)"; then
    if [[ -n "$attempted_at" ]]; then
      if ! attempted_epoch="$(date -u -d "$attempted_at" +%s 2>/dev/null)"; then
        log_error "cannot parse ${BACKUP_SERVICE_UNIT} completion timestamp"
        attempted_epoch=0
        source_ok=0
      fi
    fi
  else
    log_error "cannot read ${BACKUP_SERVICE_UNIT} completion timestamp"
    source_ok=0
  fi

  timer_state=""
  if timer_state="$(systemctl show "$BACKUP_TIMER_UNIT" --property=ActiveState --value 2>/dev/null)"; then
    [[ "$timer_state" == "active" ]] && timer_active=1
  else
    log_error "cannot read ${BACKUP_TIMER_UNIT} state"
    source_ok=0
  fi

  emit_metric "drivergo_backup_timer_active ${timer_active}"
  emit_metric "drivergo_backup_last_attempt_success ${attempt_success}"
  emit_metric "drivergo_backup_last_attempt_unixtime ${attempted_epoch}"
  [[ "$source_ok" == "1" ]]
}

collect_backup_snapshot() {
  local now latest_epoch=0 created_at created_epoch directory base age=0
  local present=0 invalid=0
  local -a candidates=()

  emit_header drivergo_backup_snapshot_present "Whether a complete allowlisted local full-backup snapshot exists." gauge
  emit_header drivergo_backup_last_snapshot_unixtime "Unix creation time from the latest complete local snapshot manifest." gauge
  emit_header drivergo_backup_snapshot_age_seconds "Age in seconds of the latest complete local snapshot." gauge
  emit_header drivergo_backup_snapshot_invalid "Number of allowlisted local snapshots that failed strict manifest/checksum verification." gauge

  now="$(date -u +%s)" || return 1
  if [[ -d "$BACKUP_ROOT" && ! -L "$BACKUP_ROOT" ]]; then
    shopt -s nullglob
    candidates=("$BACKUP_ROOT"/drivergo-????????T??????Z)
    shopt -u nullglob
    for directory in "${candidates[@]}"; do
      [[ -d "$directory" && ! -L "$directory" ]] || continue
      base="$(basename "$directory")"
      [[ "$base" =~ ^drivergo-[0-9]{8}T[0-9]{6}Z$ ]] || continue
      if ! verify_snapshot_cached "$directory" "$base"; then
        invalid="$((invalid + 1))"
        continue
      fi
      created_at="$(awk -F= '$1 == "created_at" { sub(/^[^=]*=/, ""); print; exit }' "$directory/manifest.txt")"
      [[ "$created_at" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]] || continue
      created_epoch="$(date -u -d "$created_at" +%s 2>/dev/null)" || continue
      if (( created_epoch > latest_epoch )); then
        latest_epoch="$created_epoch"
      fi
    done
  fi

  if (( latest_epoch > 0 )); then
    present=1
    age="$((now - latest_epoch))"
    (( age >= 0 )) || age=0
  fi
  emit_metric "drivergo_backup_snapshot_present ${present}"
  emit_metric "drivergo_backup_last_snapshot_unixtime ${latest_epoch}"
  emit_metric "drivergo_backup_snapshot_age_seconds ${age}"
  emit_metric "drivergo_backup_snapshot_invalid ${invalid}"
}

collect_disk_path() {
  local target="$1"
  local path="$2"
  local present=0 size=0 available=0 used_ratio=0 row

  if [[ -d "$path" ]]; then
    present=1
    row="$(df -P -B1 -- "$path" 2>/dev/null | awk 'NR == 2 { print $2, $4 }')" || return 1
    read -r size available <<<"$row"
    if [[ ! "$size" =~ ^[0-9]+$ || ! "$available" =~ ^[0-9]+$ || "$size" == "0" ]]; then
      log_error "cannot parse filesystem capacity for ${target}"
      return 1
    fi
    used_ratio="$(awk -v size="$size" -v available="$available" 'BEGIN { printf "%.6f", (size - available) / size }')" || return 1
  fi

  emit_metric "drivergo_disk_path_present{target=\"${target}\"} ${present}"
  emit_metric "drivergo_disk_size_bytes{target=\"${target}\"} ${size}"
  emit_metric "drivergo_disk_available_bytes{target=\"${target}\"} ${available}"
  emit_metric "drivergo_disk_used_ratio{target=\"${target}\"} ${used_ratio}"
}

collect_disks() {
  local source_ok=1
  emit_header drivergo_disk_path_present "Whether the allowlisted filesystem path exists." gauge
  emit_header drivergo_disk_size_bytes "Filesystem size in bytes for the allowlisted path." gauge
  emit_header drivergo_disk_available_bytes "Filesystem bytes available to an unprivileged process." gauge
  emit_header drivergo_disk_used_ratio "Filesystem used ratio for the allowlisted path." gauge
  collect_disk_path root / || source_ok=0
  collect_disk_path backup "$BACKUP_ROOT" || source_ok=0
  [[ "$source_ok" == "1" ]]
}

collect_optional_tls_certificate() {
  local enabled=0 present=0 valid=0 not_before_epoch=0 not_after_epoch=0 expiry_seconds=0
  local certificate_dates not_before not_after now

  emit_header drivergo_tls_certificate_monitoring_enabled "Whether optional local TLS certificate monitoring is configured." gauge
  emit_header drivergo_tls_certificate_present "Whether the configured local TLS certificate file is readable." gauge
  emit_header drivergo_tls_certificate_valid "Whether the configured local TLS certificate parses and its validity interval includes the current time." gauge
  emit_header drivergo_tls_certificate_not_before_unixtime "Unix not-before time of the configured local TLS certificate." gauge
  emit_header drivergo_tls_certificate_not_after_unixtime "Unix not-after time of the configured local TLS certificate." gauge
  emit_header drivergo_tls_certificate_expiry_seconds "Seconds remaining before the configured local TLS certificate expires." gauge

  if [[ -n "$TLS_CERT_FILE" ]]; then
    enabled=1
    if [[ -f "$TLS_CERT_FILE" ]]; then
      present=1
      certificate_dates="$(openssl x509 -in "$TLS_CERT_FILE" -noout -startdate -enddate 2>/dev/null || true)"
      not_before="$(awk -F= '$1 == "notBefore" { sub(/^[^=]*=/, ""); print; exit }' <<<"$certificate_dates")"
      not_after="$(awk -F= '$1 == "notAfter" { sub(/^[^=]*=/, ""); print; exit }' <<<"$certificate_dates")"
      if [[ -n "$not_before" && -n "$not_after" ]]; then
        not_before_epoch="$(date -u -d "$not_before" +%s 2>/dev/null || printf '0')"
        not_after_epoch="$(date -u -d "$not_after" +%s 2>/dev/null || printf '0')"
        now="$(date -u +%s)" || return 1
        if [[ "$not_before_epoch" =~ ^[0-9]+$ && "$not_after_epoch" =~ ^[0-9]+$ ]] &&
          (( not_before_epoch > 0 && not_after_epoch > now && not_before_epoch <= now )); then
          valid=1
        fi
        if [[ "$not_after_epoch" =~ ^[0-9]+$ ]] && (( not_after_epoch > now )); then
          expiry_seconds="$((not_after_epoch - now))"
        fi
      fi
    fi
  fi

  emit_metric "drivergo_tls_certificate_monitoring_enabled ${enabled}"
  emit_metric "drivergo_tls_certificate_present ${present}"
  emit_metric "drivergo_tls_certificate_valid ${valid}"
  emit_metric "drivergo_tls_certificate_not_before_unixtime ${not_before_epoch}"
  emit_metric "drivergo_tls_certificate_not_after_unixtime ${not_after_epoch}"
  emit_metric "drivergo_tls_certificate_expiry_seconds ${expiry_seconds}"
}

cleanup() {
  if [[ -n "$METRICS_FILE" && -e "$METRICS_FILE" ]]; then
    rm -f -- "$METRICS_FILE"
  fi
}

main() {
  local published_at lock_fd
  require_command awk || return 1
  require_command basename || return 1
  require_command chmod || return 1
  require_command date || return 1
  require_command df || return 1
  require_command dirname || return 1
  require_command docker || return 1
  require_command flock || return 1
  require_command mkdir || return 1
  require_command mktemp || return 1
  require_command mv || return 1
  require_command rm || return 1
  require_command sha256sum || return 1
  require_command stat || return 1
  require_command systemctl || return 1
  if [[ -n "$TLS_CERT_FILE" ]]; then
    require_command openssl || return 1
    validate_absolute_file_setting TLS_CERT_FILE "$TLS_CERT_FILE" || return 1
  fi
  validate_absolute_file_setting COMPOSE_FILE "$COMPOSE_FILE" || return 1
  validate_absolute_file_setting COMPOSE_ENV_FILE "$COMPOSE_ENV_FILE" || return 1
  validate_absolute_file_setting BACKUP_ROOT "$BACKUP_ROOT" || return 1
  validate_verifier || return 1
  validate_output_paths || return 1
  validate_verify_cache || return 1

  exec {lock_fd}>"$LOCK_FILE" || return 1
  if ! flock -n "$lock_fd"; then
    log_error "another collector instance holds ${LOCK_FILE}"
    return 75
  fi

  METRICS_FILE="$(mktemp "${TEXTFILE_DIR}/.drivergo_ops.prom.tmp.XXXXXX")" || return 1
  trap cleanup EXIT
  trap 'exit 129' HUP
  trap 'exit 130' INT
  trap 'exit 143' TERM

  collect_containers || COLLECTOR_SUCCESS=0
  collect_backup_unit || COLLECTOR_SUCCESS=0
  collect_backup_snapshot || COLLECTOR_SUCCESS=0
  collect_disks || COLLECTOR_SUCCESS=0
  collect_optional_tls_certificate || COLLECTOR_SUCCESS=0

  published_at="$(date -u +%s)" || {
    log_error "cannot read current time"
    COLLECTOR_SUCCESS=0
    published_at=0
  }
  emit_header drivergo_ops_collector_success "Whether every required source was read during this collector run." gauge
  emit_header drivergo_ops_collector_timestamp_seconds "Unix time when this collector output was published." gauge
  emit_metric "drivergo_ops_collector_success ${COLLECTOR_SUCCESS}"
  emit_metric "drivergo_ops_collector_timestamp_seconds ${published_at}"

  if [[ "$OUTPUT_SUCCESS" != "1" ]]; then
    return 1
  fi
  chmod 0644 -- "$METRICS_FILE" || return 1
  mv -f -- "$METRICS_FILE" "$OUTPUT_FILE" || return 1
  METRICS_FILE=""
  if [[ "$COLLECTOR_SUCCESS" != "1" ]]; then
    log_error "one or more required sources could not be read"
    return 1
  fi
}

main "$@"
