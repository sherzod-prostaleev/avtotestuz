#!/usr/bin/env bash
# Validate candidate preflight entirely with command fixtures; no daemon, SSH
# host, container, migration, or nginx configuration is touched.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$ROOT/deploy/candidate-app.sh"
test_root="$(mktemp -d)"
trap 'rm -rf -- "$test_root"' EXIT

mkdir -p "$test_root/bin" "$test_root/app"
cp "$ROOT/deploy/app.prod.env.example" "$test_root/app/app.env"
printf '%s\n' \
  'MINIO_ROOT_USER=test-user' \
  'MINIO_ROOT_PASSWORD=test-password' \
  >>"$test_root/app/app.env"
chmod 600 "$test_root/app/app.env"

cat >"$test_root/bin/docker" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%q ' "$@" >>"${MOCK_DOCKER_LOG:?}"
printf '\n' >>"${MOCK_DOCKER_LOG:?}"
case "${1:-}" in
  compose) exit 0 ;;
  network)
    [[ "${2:-}" == inspect ]]
    exit 0
    ;;
  image)
    [[ "${2:-}" == inspect ]]
    exit 0
    ;;
esac
exit 1
EOF

cat >"$test_root/bin/ssh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%q ' "$@" >>"${MOCK_SSH_LOG:?}"
printf '\n' >>"${MOCK_SSH_LOG:?}"
for arg in "$@"; do
  if [[ "$arg" == bash ]]; then
    cat >/dev/null
    printf 'backup snapshot gate: ok (snapshot=drivergo-20260810T063020Z age_seconds=1)\n'
    exit 0
  fi
done
echo "candidate preflight did not invoke the read-only remote checker" >&2
exit 1
EOF
chmod 0755 "$test_root/bin/docker" "$test_root/bin/ssh"

docker_log="$test_root/docker.log"
ssh_log="$test_root/ssh.log"
: >"$docker_log"
: >"$ssh_log"

output="$(
  cd "$test_root/app"
  env \
    PATH="$test_root/bin:$PATH" \
    MOCK_DOCKER_LOG="$docker_log" \
    MOCK_SSH_LOG="$ssh_log" \
    DRIVERGO_CANDIDATE_ENV_FILE="$test_root/app/app.env" \
    CANDIDATE_API_IMAGE="registry.example/api@sha256:$(printf 'a%.0s' {1..64})" \
    CANDIDATE_WEB_IMAGE="registry.example/web@sha256:$(printf 'b%.0s' {1..64})" \
    CANDIDATE_BACKUP_HOST=root@192.0.2.10 \
    CANDIDATE_BACKUP_ALLOWED_HOSTS=root@192.0.2.10 \
    CANDIDATE_BACKUP_ROOT=/var/backups/drivergo/full \
    "$SCRIPT" preflight
)"

grep -Fq 'candidate preflight ok; no container started and nginx was not changed' <<<"$output"
grep -Fq 'required environment names:' <<<"$output"
grep -Fq 'CANDIDATE_API_IMAGE' <<<"$output"
grep -Fq 'CLIENT_IP_ASSERTION_SECRET' <<<"$output"
! grep -Fq 'up -d' "$docker_log"
! grep -Fq 'nginx' "$docker_log"
grep -Fq -- '-o BatchMode=yes' "$ssh_log"
grep -Fq -- '-o StrictHostKeyChecking=yes' "$ssh_log"

if env \
  PATH="$test_root/bin:$PATH" \
  MOCK_DOCKER_LOG="$docker_log" \
  MOCK_SSH_LOG="$ssh_log" \
  DRIVERGO_CANDIDATE_ENV_FILE="$test_root/app/app.env" \
  CANDIDATE_API_IMAGE="registry.example/api:mutable" \
  CANDIDATE_WEB_IMAGE="sha256:$(printf 'b%.0s' {1..64})" \
  CANDIDATE_BACKUP_HOST=root@192.0.2.10 \
  CANDIDATE_BACKUP_ALLOWED_HOSTS=root@192.0.2.10 \
  CANDIDATE_BACKUP_ROOT=/var/backups/drivergo/full \
  "$SCRIPT" preflight >/dev/null 2>&1; then
  echo "expected mutable image preflight rejection" >&2
  exit 1
fi

printf 'candidate preflight guards: ok\n'
