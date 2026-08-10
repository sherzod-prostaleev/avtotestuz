# Home PC off-host pull — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Interim encrypted snapshot pull from VPS → operator CachyOS PC with verify + user timer.

**Architecture:** `pull_offhost.sh` (SSH list + rsync resume + verify); interim VPS unit with `REQUIRE_OFFSITE_BACKUP=0`; user systemd timer on PC.

**Tech stack:** bash, ssh, rsync, flock, existing `verify_snapshot.sh` / `lib.sh`.

## File map

- `scripts/backup/pull_offhost.sh` — pull orchestrator
- `scripts/backup/offhost-pull.env.example` — PC env template
- `scripts/backup/systemd/drivergo-offhost-pull.{service,timer}` — user units
- `deploy/systemd/drivergo-backup-homepc.{service,timer}` — interim VPS schedule
- `scripts/backup/test_backup_scripts.sh` — mocked SSH/rsync contracts
- `scripts/backup/README.md`, `DEVOPS-REMEDIATION-HANDOFF.md`, `Makefile`

## Tasks

1. Implement `pull_offhost.sh` + env example — **DONE**
2. Add PC user systemd units + interim VPS units — **DONE**
3. Extend backup contract tests — **DONE** (48/48)
4. Document install/run; update handoff P1-A interim status — **DONE**
5. Run `./scripts/backup/test_backup_scripts.sh` — **DONE**

## Operational activation (2026-08-10)

- PC timer enabled; age identity on PC only
- VPS `drivergo-backup-homepc.timer` enabled with `AGE_RECIPIENT`
- Snapshot `drivergo-20260810T063020Z` pulled + verified + decrypt smoke

**Plan status: COMPLETE** (cloud rclone off-site remains under handoff P1-A close-out)
