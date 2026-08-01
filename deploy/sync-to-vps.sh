#!/usr/bin/env bash
# Sync product tree to VPS without agent junk / docs / local caches.
# Usage:
#   ./deploy/sync-to-vps.sh [user@host]
# Default host: root@89.117.59.137
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
HOST="${1:-root@89.117.59.137}"
DEST="${DEPLOY_PATH:-/opt/drivergo}"
EXCLUDE="${ROOT}/deploy/rsync-exclude.txt"

# Runtime code is mirrored exactly so removed security-sensitive routes cannot
# survive a deploy. Host-only secrets, backups, caches and Docker volumes are
# excluded/protected by the filter file and explicit app.env rule.
rsync -az \
  --delete-delay \
  --exclude-from="${EXCLUDE}" \
  --filter='P deploy/app.env' \
  -e 'ssh -o StrictHostKeyChecking=accept-new' \
  "${ROOT}/" \
  "${HOST}:${DEST}/"

echo "synced → ${HOST}:${DEST}"
echo "protected on host: ${DEST}/deploy/app.env"
