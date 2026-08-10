# Home PC off-host backup pull — design

Date: 2026-08-10  
Status: approved (operator: Sherzod)

## Problem

Production needs a second copy of encrypted full-stack snapshots outside the
VPS. A cloud object store is not provisioned yet. The operator’s CachyOS PC is
an acceptable interim off-host target when the network is up.

## Non-goals

- Not a substitute for provider-backed rclone off-site (`REQUIRE_OFFSITE_BACKUP=1`).
- Not automatic paging, PITR, or object lock.
- Do not open inbound ports on the PC; the PC always pulls over SSH.
- Do not store age private identity on the VPS.

## Approach (chosen)

**PC pull over SSH/rsync**, scheduled by a user systemd timer:

1. VPS creates encrypted snapshots under `/var/backups/drivergo/full` with
   interim unit `REQUIRE_OFFSITE_BACKUP=0` (until a real remote exists).
2. PC, when online, lists complete remote snapshots and rsyncs missing ones into
   a local `0700` directory via `.partial-*` → atomic rename after
   `verify_snapshot.sh`.
3. Offline PC → pull fails closed and retries on the next timer tick; VPS
   backup itself still succeeds.

## Trust claims

Honest label: **home PC off-host pull**.  
Do not claim automatic provider off-site, zero data loss, or paging.

## Defaults

- SSH: same allowlisted default as `deploy/sync-to-vps.sh` (`DEPLOY_HOST` /
  `OFFHOST_SSH_HOST`).
- Remote root: `/var/backups/drivergo/full`
- Local root: `$HOME/drivergo-offhost/full`
- Timer: every 30 minutes, `Persistent=true`
