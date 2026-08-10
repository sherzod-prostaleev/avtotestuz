# DriverGo DevOps remediation handoff

Updated: 2026-08-10 (Asia/Tashkent)  
Branch: `cursor/integrate-support-chat`  
Baseline/live deploy marker: `aed74d02f7c570ae2db7c4b55bd14b138c85c487`

## Non-negotiable scope

- Do **not** change SSH/sshd root login, password login, fail2ban, firewall SSH rules, or any SSH credential.
- Do **not** change admin TOTP behavior, enforcement, enrollment, secrets, or `ADMIN_TOTP_ENFORCE`.
- Never print, commit, copy into logs, or ask the user to paste production secrets into chat.
- Preserve the B2B station's Windows 7 contract: Go 1.20, `CGO_ENABLED=0`, `GOOS=windows`, `GOARCH=386`.
- No production restart, migration, nginx switch, destructive storage action, or deploy until a fresh backup, restore evidence, immutable images, green tests, and explicit rollout approval exist.

## Current honest status

Repository remediation source work is advanced; **app images are still not rolled out**. Live DriverGo still uses the previous known-good app images and deploy marker `aed74d0` unless noted below.

Operational progress since the original handoff write-up:

- Interim encrypted VPS backups are scheduled via `drivergo-backup-homepc.timer`
  (`REQUIRE_OFFSITE_BACKUP=0`) with `/etc/drivergo/backup.env` containing
  `AGE_RECIPIENT` only (private age identity on the operator PC).
- Successful VPS snapshot `drivergo-20260810T063020Z` was pulled to the PC
  (`~/drivergo-offhost/full`), checksum-verified, and age-decrypt smoked.
- PC `drivergo-offhost-pull.timer` is enabled (pull when online).
- Humo spool ENOSPC now fails the critical supervisor (unit-tested).
- Candidate nginx switch retains the prior include until reload commits;
  failure/signal rollback covered by `deploy/tests/switch-app-slot_test.sh`.
- Win7 binary gate parses modern govuln JSON; `x/sys` raised to v0.30.0
  (Go 1.20 max); GOVERNANCE.md documents the 59-ID allowlist.
- MinIO/prod digest Trivy (2026-08-10): **HIGH/CRITICAL present** (incl.
  CRITICAL CVE-2026-33322). `minio/minio:latest` resolves to the **same**
  digest — no safer upstream digest available to pin yet.
- Monitoring candidate digests have a fresh zero-HIGH/CRITICAL `linux/amd64`
  Trivy result, but no owned webhook is configured; do **not** deploy the
  monitoring bundle.

Still true:

- No provider rclone off-site, no `REMOTE_COMPLETE` cloud path, no paging.
- No production nginx slot switch / app candidate rollout yet.

## Seven remediation stages implemented in source

1. **Encrypted full-stack backup and DR**
   - Atomic Postgres/Redis/MinIO/Humo snapshots, mandatory `age` and off-site mode for production, strict manifests/checksums, retention minimum, invalid-local quarantine, and uncertain-remote preservation.
   - Real component/full restore drills use allowlisted scratch resources and fail if cleanup fails.

2. **MinIO privacy, compatibility, and readiness**
   - Private `support-attachments` bucket with legacy authenticated `media/support/*` fallback.
   - Public policy is limited to `media/images/*`; support objects are not anonymous.
   - Target bucket versioning is enabled. Migration is dry-run by default, source-retaining, checksum-validating, collision-failing, and only treats exact S3 not-found codes as missing.
   - API readiness includes primary support object storage.

3. **Humo and Redis resilience**
   - Humo has a durable SQLite WAL/FULL spool, bounded health endpoint, critical-task supervision, non-root/read-only container, persistent volume, and restart-on-worker-death behavior.
   - Hashed Python lock is enforced. Runtime moved to digest-pinned Alpine after current Debian base CVEs were found.
   - Redis/MinIO/Humo component restore drills exist; Redis AOF policy is explicit.

4. **Availability-safe deployment path**
   - Sync defaults to dry-run, validates exact host/path, clean tree, remote sentinels, protected env mode, Compose, and running health.
   - Rollback snapshot is captured under `.partial-*`, receives metadata/completion digest, then atomically renamed; incomplete snapshots are rejected.
   - App-only candidate stack uses immutable refs, one API/one web, no duplicate Humo/stateful service, health gates, shared slot lock, `restart: unless-stopped`, and atomic nginx switch/rollback.

5. **CI/CD and supply-chain gates**
   - Independent Go/station/npm/Python scans, weekly schedule, hashed Python deps, npm overrides, CodeQL, pinned actions/images, container build and Trivy HIGH/CRITICAL gates.
   - Station CI now builds the exact Go 1.20 Windows/386 binary, runs binary-mode govulncheck, and fails on findings outside the reviewed Win7 exception allowlist.
   - E2E secret-bearing traces are disabled/not uploaded. Load test is protected, allowlisted, and bounded.
   - Remaining Win7 binary scanner gap is listed below.

6. **Windows 7 B2B compatibility**
   - Final image evidence: PE32 Windows 6.01 Intel 386, Go 1.20.14, CGO off, Windows/386.
   - Exact Go 1.20 container tests, vet, and Windows/386 build passed.
   - HTTP/1.1-only bounded transport, TLS 1.2 minimum, disabled TLS session tickets, and connection/header limits reduce EOL-runtime exposure.

7. **Observability and incident readiness**
   - Low-cardinality API latency histogram, Prometheus rules, blackbox probes, node textfile metrics, backup integrity/age/timer alerts, Alertmanager config, SLO/RPO/RTO runbook.
   - Alertmanager has a dedicated outbound network and reads its webhook through a numeric supplemental group from a root-owned mode-0640 file.
   - Backup full-hash results are cached for 25 hours, preventing one-minute multi-GB rehash loops; scrubs run at low CPU/I/O priority.

## Verification already passed

- Full backend integration suite and `go vet`: PASS.
- Frontend lint, typecheck, 108 unit files / 577 tests, production build: PASS.
- Playwright: 56 passed, 32 credential-dependent skips, 0 failed.
- Exact Go 1.20 station test/vet/Windows-386 build: PASS.
- Backup/restore mock/contract suite: 42/42 PASS.
- Monitoring contract suite: 17/17 PASS.
- Compose contracts: 8/8 PASS.
- Sync safety contract: PASS.
- Humo tests: 4/4 PASS; dependency lock: 2 direct / 14 total; pip-audit: 0 known vulnerabilities.
- `npm audit --audit-level=high`: 0 vulnerabilities.
- Backend and station source `govulncheck`: 0 reachable findings; the shipped binary is additionally checked in binary mode against the reviewed Win7 exception list.
- Final runtime Trivy scan: API (excluding embedded Win7 EXE) 0 HIGH/CRITICAL; web 0; Humo Alpine 0.
- `actionlint`, `bash -n`, `gofmt`, `go mod tidy -diff`, and `git diff --check`: PASS at the last completed gate.
- Forbidden SSH/TOTP diff search: no change found.

## Remaining work, ordered by severity

### P1-A — Activate real automatic encrypted off-site backup

**Interim (2026-08-10, activated):** home PC pull + VPS interim timer are live.
- PC: `drivergo-offhost-pull.timer` enabled; age private identity only on PC
  under `~/drivergo-offhost/age/` (never on VPS).
- VPS: `/etc/drivergo/backup.env` has `AGE_RECIPIENT` only;
  `drivergo-backup-homepc.timer` enabled; successful run
  `drivergo-20260810T063020Z` pulled and verified on PC (incl. age decrypt smoke).
Honest claim: operator-PC off-host copy when online — **not** provider-backed
off-site. Cloud rclone path remains open below.

**Prep-ready in source (2026-08-10); no provider is configured:** the operator
runbook is `deploy/backup-offsite-setup.md`, and
`scripts/backup/offsite_preflight.sh` performs non-secret local checks for
`AGE_RECIPIENT`, an allowlisted `RCLONE_REMOTE` prefix, and mode-0600 rclone
configuration. Its live provider listing is explicit and may be skipped only
for a dry configuration check. These additions do not provision an account,
write a credential, contact a real remote from CI, or enable a production
timer.

External input remains required for real P1-A close-out: choose an
S3/R2/B2/SFTP-compatible provider and securely provision the endpoint/remote,
bucket/path, access ID/secret, and age recipient/identity policy. Follow the
runbook in order: configure root-owned mode-0600
`/etc/drivergo/rclone.conf`, extend root-owned mode-0600 `backup.env` with
`RCLONE_CONFIG` and `RCLONE_REMOTE`, run dry then live preflight, produce a
fresh encrypted four-component snapshot, verify the remote `REMOTE_COMPLETE`
and checksum manifest, and complete a remote-download restore drill. Only
then disable `drivergo-backup-homepc.timer` and enable
`drivergo-backup.timer`; its `drivergo-backup.service` invokes committed
`backup_all.sh --production`. Observe a scheduled success and test a
controlled failure alert before claiming automatic provider-backed off-site
backup.

Never substitute a second directory on the same VPS and call it off-site.
Home PC pull is allowed as an interim second host only.

### P1-B — Activate real alert delivery

**Still blocked on external webhook input; image gate now has clean candidate
digests.** A fresh 2026-08-10 Trivy vulnerability scan of locally pulled
`linux/amd64` upstream release images found **0 HIGH / 0 CRITICAL** in each
exact immutable reference:

- `quay.io/prometheus/prometheus@sha256:63805ebb8d2b3920190daf1cb14a60871b16fd38bed42b857a3182bc621f4996` (`v3.5.0`)
- `quay.io/prometheus/alertmanager@sha256:27c475db5fb156cab31d5c18a4251ac7ed567746a2483ff264516437a39b15ba` (`v0.28.1`)
- `quay.io/prometheus/blackbox-exporter@sha256:92e05d5fe0df01d3980518dc42f07b778a8997048b57392ebc7f7391ebd7bb06` (`v0.26.0`)
- `quay.io/prometheus/node-exporter@sha256:d00a542e409ee618a4edc67da14dd48c5da66726bbd5537ab2af9c1dfc442c8a` (`v1.9.1`)

The older candidate report was superseded by this exact local-digest scan.
Re-scan the exact selected digests immediately before deployment with a current
Trivy database via `deploy/monitoring/verify_images.sh`; the preflight also
requires the protected webhook file and rejects a non-`linux/amd64` image,
every HIGH, and every CRITICAL finding. Never substitute a mutable tag or a
scan of another platform. The committed environment remains empty, so
validation fails closed until all four exact digests are inserted into a
root-owned host environment.

Operator action still required: choose and configure an owned HTTPS receiver
outside Git using `deploy/monitoring/webhook.env.example` as the path/GID-only
contract; then validate receiver-file permissions and prove a labelled
synthetic firing and resolved notification. Do not create a placeholder URL or
deploy the monitoring stack before this evidence exists.

### P1-C — Govern the Win7 binary vulnerability exceptions

**In-repo governance advanced (2026-08-10):**

1. `golang.org/x/sys` raised to highest Go-1.20-compatible `v0.30.0`. Fix for
   `GO-2026-5024` needs `v0.44.0` (Go 1.25 tooling) — **not** adoptable under
   the Win7 contract.
2. `backend/station/security/GOVERNANCE.md` + allowlist document owner, review
   date, and exit conditions for the 59 accepted IDs.
3. `verify_binary_govuln.py` accepts pretty-printed govulncheck JSON streams.

Still open (external/hardware):

1. Real Windows 7 VM/machine smoke (`-selftest`, DPAPI, registry, owned HTTPS).
2. Authenticode signing/reputation before any long-term compatibility claim.

### P1-D — Remaining transition and supply-chain blockers

- Humo callback/spool ENOSPC supervision: **DONE** in source + unit tests
  (process exits; Docker `unless-stopped` can restart). Live restart proof on
  VPS still optional.
- Candidate switch transaction + shell rollback proof: **DONE**
  (`deploy/tests/switch-app-slot_test.sh`). Live production nginx rollback still
  intentionally unexercised until P1-F.
- MinIO release: **scanned**. Exact digest
  `minio/minio@sha256:14cea493d9a34af32f524e538b8346cf79f3321eff8e708c1e2960462bd8936e`
  has HIGH/CRITICAL (incl. CRITICAL CVE-2026-33322). `minio/minio:latest`
  currently resolves to the same digest — **upgrade blocked until upstream
  publishes a different clean digest**. Do not guess.
- Monitoring: still blocked (see P1-B).

### P1-E — Reduce true data-loss window

**Fail-closed scaffolding landed (2026-08-10), production enablement is not
approved:** `pitr_archive_wal.sh`, `pitr_verify_wal.sh`, and
`pitr_restore_to_time_drill.sh` provide an explicit opt-in archive helper,
archive integrity verification, and a network-isolated scratch-only recovery
drill. No live PostgreSQL configuration, VPS service, systemd unit, or app
rollout was changed.

The operator must separately approve/configure the archive setting, immutable
helper runtime, encrypted off-host WAL/base-backup storage, and run a timed
isolated-host drill. Until then the honest RPO remains the existing snapshot
interval; do not claim WAL/PITR coverage.

Still open: WAL/PITR, provider object lock, storage-native MinIO consistency,
business RPO/RTO sign-off after timed isolated-host recovery.

### P1-F — Safely land and roll out the repository changes

1. Review the complete uncommitted diff; do not discard unrelated/user work.
2. Re-run all gates above and CI on a reviewed commit.
3. Build/push API/web/Humo images once, record registry digests, scan those exact digests, and never promote mutable tags.
4. Take/verify a fresh off-site snapshot before any production change.
5. Start app-only candidate with expand/contract acknowledgement; verify API/web/MinIO/support/Humo/payment smoke without duplicating Humo.
6. Switch nginx atomically, observe errors/latency/restarts, and keep stable slot healthy for rollback.
7. Do not delete legacy MinIO objects or old images/backups in the same change window.

## Copy-paste prompt for the next AI

```text
Act as a principal SRE, security engineer, database reliability engineer, and adversarial reviewer. Work in /home/sher/Рабочий стол/avtotest. First read DEVOPS-REMEDIATION-HANDOFF.md completely, then inspect the current git diff and live state read-only. Do not trust claims without evidence.

NON-NEGOTIABLE EXCLUSIONS:
- Never modify SSH/sshd root login, password login, SSH credentials, fail2ban, or SSH firewall rules.
- Never modify admin TOTP behavior/config/secrets or ADMIN_TOTP_ENFORCE.
- Never print or commit credentials. Ask only for the chosen provider/receiver and instruct the operator to place secrets in protected host files.
- Preserve B2B Windows 7: backend/station must remain Go 1.20, CGO_ENABLED=0, GOOS=windows, GOARCH=386. Do not silently upgrade its toolchain.
- No live deploy/restart/nginx switch/destructive storage operation until all release gates and a fresh verified off-site restore point pass. Default to dry-run and copy-only operations.

OBJECTIVE:
Finish only the remaining P1 items in the handoff, with zero avoidable downtime/data loss. Review the implemented Win7 binary gate and its 59-entry EOL exception list, then prepare provider-specific off-site and alerting activation; do not invent credentials or call local storage “off-site”. Add PostgreSQL PITR/object-lock work as a separately reviewed change, not mixed into an app rollout.

REQUIRED ENGINEERING METHOD:
1. Extract requirements as REQ-001... and map each to file/line evidence and automated verification.
2. Run an adversarial review before edits and again after the tree is frozen.
3. Preserve all existing user changes; use apply_patch; no destructive git commands.
4. Every deletion/prune/rollback must distinguish not-found, corruption, permission, timeout, and transport failure. Uncertain remote data must be preserved, never auto-purged.
5. Every deploy artifact must be immutable by registry digest/content ID, scanned as the exact shipped artifact, with provenance/SBOM where possible.
6. For station CI: build exact Go-1.20 Windows/386 EXE, scan that binary with current govulncheck JSON, enforce a reviewed allowlist that fails on new findings, publish evidence, and retain a real Win7 smoke gate. Source-mode scanning with a newer stdlib is insufficient.
7. For monitoring: verify webhook readability by the actual non-root Alertmanager UID/GID and verify outbound HTTPS egress. Use only zero-HIGH/CRITICAL reviewed immutable images. Prove end-to-end firing and resolved delivery.
8. For backup: configure a real remote, require age encryption, validate remote completion/hash, restore from the remote copy, enable the systemd timer, and observe a scheduled run. Never expose secrets in output.
9. For rollout: fresh backup -> restore evidence -> green CI -> exact image scans -> candidate health -> atomic switch -> observation -> rollback proof. Never run a second Humo watcher.
10. Re-check that SSH/TOTP files/settings have no diff before handoff.

MANDATORY FINAL REPORT:
- Outcome first; exact implemented vs remaining.
- Tests with commands and pass/fail counts.
- Live services/restart counters before and after.
- Backup snapshot ID, remote verification, restore evidence, RPO/RTO limits.
- Windows 7 build metadata and all accepted vulnerability exceptions.
- Files changed, risks, rollback procedure, and explicit blockers.
- Never claim 10/10, automatic off-site, paging, zero data loss, or Win7 long-term security without direct evidence.
```

## Highest-attention files

- `scripts/backup/backup_all.sh`, `prune_backups.sh`, `verify_snapshot.sh`, `full_restore_drill.sh`, `pull_offhost.sh`
- `deploy/systemd/drivergo-backup.*`, `deploy/systemd/drivergo-backup-homepc.*`, `deploy/backup.env.example`
- `scripts/backup/systemd/drivergo-offhost-pull.*`, `scripts/backup/offhost-pull.env.example`
- `deploy/migrate-support-bucket.sh`, `deploy/docker-compose.prod.yml`
- `deploy/sync-to-vps.sh`, `candidate-app.sh`, `switch-app-slot.sh`
- `deploy/monitoring/docker-compose.monitoring.yml`, `validate_env.sh`, `write_textfile_metrics.sh`, `RUNBOOK.md`
- `.github/workflows/ci.yml`, `.github/workflows/codeql.yml`
- `backend/Dockerfile`, `backend/station/go.mod`, `backend/station/internal/netclient/`
- `services/humo-watcher/watcher.py`, `Dockerfile`, `requirements.lock`

This handoff intentionally favors fail-closed behavior and truthful operational claims over a nominal 10/10 score.
