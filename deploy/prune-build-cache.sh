#!/usr/bin/env bash
# Reclaim Docker build cache that no build will ever consult again.
#
# BuildKit keeps every layer it has ever built and never reclaims any of it on
# its own. Each `docker compose build api web` adds more, and nothing here runs
# often enough for that to look like a problem until it is one: on 2026-08-27
# this VPS had 503 cache entries holding 30 GB, none of them in use, and the
# disk was 77% full. Pruning them took it to 35%.
#
# The scare that came with it is worth writing down, because the obvious
# suspect was innocent. `docker system df` reported "Images 34 GB, 28.89 GB
# reclaimable (84%)" and pointed straight at 64 accumulated image tags. It was
# wrong: with the build cache gone those same images turned out to occupy 5.25
# GB in total, 80 MB of it reclaimable, because their layers are shared with
# the images actually running. Deleting old rollback tags would have freed
# almost nothing and cost the ability to roll back. When this disk fills again,
# read the Build Cache row, not the Images row.
#
# Keeps the last KEEP window so an incremental build stays fast -- the point is
# to bound the cache, not to make every deploy start from nothing.
set -euo pipefail

KEEP="${BUILD_CACHE_KEEP:-168h}" # one week

if ! command -v docker >/dev/null 2>&1; then
  echo "prune-build-cache: docker is not installed here" >&2
  exit 1
fi

before_disk="$(df -h --output=used,avail,pcent / | tail -1 | tr -s ' ')"
before_cache="$(docker system df --format '{{.Type}}\t{{.Size}}' 2>/dev/null |
  awk -F'\t' '$1 == "Build Cache" {print $2}')"

# `-f` because this runs unattended; `--filter until=` is what keeps it honest
# -- without it this would also throw away the cache the next build wants.
docker builder prune --filter "until=$KEEP" -f

after_disk="$(df -h --output=used,avail,pcent / | tail -1 | tr -s ' ')"
after_cache="$(docker system df --format '{{.Type}}\t{{.Size}}' 2>/dev/null |
  awk -F'\t' '$1 == "Build Cache" {print $2}')"

printf 'prune-build-cache: keep=%s  cache %s -> %s  disk [%s] -> [%s]\n' \
  "$KEEP" "${before_cache:-?}" "${after_cache:-?}" "$before_disk" "$after_disk"
