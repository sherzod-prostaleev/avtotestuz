# P1-F rollout checklist

This is an ordered, fail-closed checklist for the P1-F application rollout.
It does not authorize a production change by itself. Stop at any failed,
stale, or unverified gate. Do not restart the production stack, run a
migration, switch nginx, remove an image, or delete legacy MinIO objects in
this window.

## 1. Freeze and review the release

- [ ] Preserve unrelated work and review the complete uncommitted diff.
- [ ] Create and review a commit containing only the intended release changes.
- [ ] Re-run the required local contract, backup, backend, station, frontend,
  and CI gates on that commit.
- [ ] Record the commit SHA, the previous live deploy marker, and a rollback
  owner.

## 2. Produce immutable, scanned artifacts

- [ ] Build the API, web, and Humo images once from the reviewed commit.
- [ ] Push them to the registry and record the resulting `@sha256:<64-hex>`
  digests. A tag alone is not a release artifact.
- [ ] Scan those exact digests and retain the scan/provenance evidence.
- [ ] Do not promote an image with unresolved HIGH or CRITICAL findings.
- [ ] Keep previous known-good image digests available for rollback.

## 3. Prove a fresh recovery point

- [ ] Take a new encrypted four-component snapshot.
- [ ] Verify its checksum manifest locally and on the off-host copy.
- [ ] Record the snapshot ID, off-host completion evidence, and a successful
  restore-drill report. A snapshot on the same VPS is not off-site evidence.
- [ ] Confirm the snapshot age meets the approved RPO before continuing.

## 4. Run the candidate preflight (dry-run)

Run this **on the VPS** after the immutable API and web images are already
present. It reads the protected `deploy/app.env` but prints only required
variable names, never values:

```bash
CANDIDATE_API_IMAGE='registry.example/drivergo-api@sha256:<64-hex>' \
CANDIDATE_WEB_IMAGE='registry.example/drivergo-web@sha256:<64-hex>' \
./deploy/candidate-app.sh preflight
```

The one command validates:

- candidate Compose configuration;
- required environment-variable names and mode `0600` on `deploy/app.env`;
- both candidate references use an immutable digest/content ID and are present
  locally;
- the shared production data network exists;
- via strict-host-key SSH, a fresh encrypted VPS backup snapshot has valid
  metadata and a checksum index. The default freshness limit is 26 hours;
  set `CANDIDATE_BACKUP_MAX_AGE_SECONDS` only to the approved RPO limit.

It starts no containers and does not modify nginx. This VPS check proves only
the local snapshot metadata gate; retain the separate off-host verification and
restore-drill evidence from gate 3.

## 5. Start and validate the app-only candidate

- [ ] Verify the preflight output is green and matches the recorded image
  digests.
- [ ] Confirm all schema changes are expand/contract compatible with the
  current stable API.
- [ ] Start exactly one candidate API and one candidate web process only:

```bash
CANDIDATE_EXPAND_CONTRACT_ACK=1 ./deploy/candidate-app.sh up --apply
```

- [ ] Verify API readiness, web locale response, MinIO/support attachment,
  payment, and Humo smoke checks. Do not run a second Humo watcher.
- [ ] Confirm stable remains the active nginx slot and candidate traffic is
  still loopback-only.

## 6. Explicitly authorize and switch traffic

This is the first step that can change production traffic. Obtain explicit
rollout approval only after gates 1–5 are recorded green.

- [ ] Dry-run the switch first:

```bash
./deploy/switch-app-slot.sh --to candidate
```

- [ ] Confirm candidate API and web probes are green and inspect the upstream
  diff.
- [ ] Perform the atomic switch:

```bash
./deploy/switch-app-slot.sh --to candidate --apply
```

The script owns a slot lock, validates nginx before reload, and restores the
previous include if validation/reload/interruption fails.

## 7. Observe and retain rollback

- [ ] Observe errors, latency, health, restart counts, payment flow, and
  support-object access for the approved window.
- [ ] Keep the stable slot, previous image digests, and recovery point intact.
- [ ] If the candidate regresses, roll traffic back without stopping it:

```bash
./deploy/switch-app-slot.sh --to stable --apply
```

- [ ] Do not delete old images, backups, or legacy MinIO objects in this
  rollout window.
