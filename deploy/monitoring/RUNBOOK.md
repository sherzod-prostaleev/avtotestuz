# Driver Go monitoring and incident runbook

## Operational status and trust boundary

This bundle evaluates alerts in Prometheus and connects a hardened,
loopback-only Alertmanager to one operator-owned HTTPS webhook. The secret URL
is read from a root-owned, dedicated-group-readable mode-0640 file and never stored in Git or Compose
environment output. A rule visible as `firing`, or an Alertmanager container
that is merely healthy, is not evidence that a human received it: until a real
external route, receiver test, escalation policy, and periodic synthetic page
are verified, alert delivery/paging is not operational.

Prometheus has no application authentication and is bound to `127.0.0.1`.
Keep it loopback-only and use an approved tunnel when an operator needs the UI.
The collector runs briefly as root because Docker's Unix socket is effectively
root-equivalent even for read queries. Keep the deployed script and unit
root-owned and non-writable by the application account. No monitoring
container receives that socket. The unit points Docker CLI configuration at
its private runtime directory, so registry credentials from root's home are not
needed or exposed for local `ps`/`inspect` reads.

Snapshot payload hashing is cached for 25 hours using a fingerprint of the
manifest, checksum file, and file metadata. The one-minute collector therefore
does not continuously reread every retained MinIO/PostgreSQL payload; a new,
changed, or expired snapshot is fully verified at most once per cache window.
The scrub runs at low CPU/I/O priority and may lengthen that collector run.

Application request counters and latency are process-local and reset on API
restart. JSON Docker logs rotate at 10 MB times three per container. Sentry is
optional error reporting, not an availability page, and distributed tracing
is not enabled. The optional local certificate check validates parseability and
the not-before/not-after time window, not issuer trust or the certificate
actually served. The public blackbox probe performs normal TLS verification.
Durable off-host logs, trace correlation, and an independent external probe
remain open controls.

## Installation and validation

Prerequisites: a running production Compose project that created the external
`drivergo_app` network, Docker Compose, systemd, `flock`, GNU `date`/`df`, and
OpenSSL only when a local origin certificate path is configured.

1. Review and pull approved upstream image versions, then record the immutable
   `RepoDigest` values. Do not use a tag as the deployed value.
2. Create a dedicated receiver in the approved incident provider. Write its
   single HTTPS webhook URL to `/etc/drivergo/alert-webhook.url`, root-owned,
   group `65534`, mode `0640`; set `ALERT_WEBHOOK_GID=65534`. Compose grants
   only that supplemental group to the non-root Alertmanager process. Do not
   reuse application/payment/Telegram credentials.
3. Create `/etc/drivergo/monitoring-compose.env` from
   `monitoring.env.example`, populate all four digest references, the absolute
   webhook-file path, and optional loopback ports; set it root-owned mode
   `0600`.
4. If local origin-certificate monitoring is wanted, create the optional
   root-owned `/etc/drivergo/monitoring.env` containing only an absolute path,
   for example `TLS_CERT_FILE=/etc/letsencrypt/live/example/fullchain.pem`.
   Public certificate monitoring works through blackbox without this file.
5. Install the collector script and systemd units from the same reviewed Git
   revision as the monitoring configuration.

Example operator sequence (review paths and image sources first):

```bash
install -d -m 0755 /var/lib/drivergo-monitoring/textfile
install -d -m 0700 /etc/drivergo
install -d -o root -g root -m 0755 /usr/local/libexec
install -o root -g root -m 0755 deploy/monitoring/write_textfile_metrics.sh \
  /usr/local/libexec/drivergo-monitoring-collector
install -m 0644 deploy/monitoring/systemd/drivergo-monitoring-collector.service \
  /etc/systemd/system/
install -m 0644 deploy/monitoring/systemd/drivergo-monitoring-collector.timer \
  /etc/systemd/system/

deploy/monitoring/validate_env.sh /etc/drivergo/monitoring-compose.env
docker compose --env-file /etc/drivergo/monitoring-compose.env \
  -f deploy/monitoring/docker-compose.monitoring.yml config --quiet
# With the already-pulled, digest-pinned Prometheus image selected above:
export PROMETHEUS_IMAGE='registry/repository@sha256:reviewed_digest'
docker run --rm --network none --read-only \
  -v "$PWD/deploy/monitoring:/etc/prometheus:ro" \
  --entrypoint /bin/promtool "$PROMETHEUS_IMAGE" \
  check config /etc/prometheus/prometheus.yml
# Run the matching digest-pinned Alertmanager image with `amtool
# check-config /etc/alertmanager/alertmanager.yml` before starting the stack.

systemctl daemon-reload
systemctl enable --now drivergo-monitoring-collector.timer
systemctl start drivergo-monitoring-collector.service
docker compose --env-file /etc/drivergo/monitoring-compose.env \
  -f deploy/monitoring/docker-compose.monitoring.yml up -d
```

These commands are documentation, not actions performed by the repository
tests. The example environment intentionally fails validation while image
references are empty. Missing provider credentials do not cause outbound
delivery attempts because no delivery component is configured. When promtool
is not installed on the host, run the same checks in an isolated validation
job before deployment; a successful Compose parse does not validate PromQL.

Validate after installation:

```bash
systemctl status drivergo-monitoring-collector.timer
systemctl status drivergo-monitoring-collector.service
journalctl -u drivergo-monitoring-collector.service --since today
sed -n '1,240p' /var/lib/drivergo-monitoring/textfile/drivergo_ops.prom
curl -fsS http://127.0.0.1:9091/-/ready
curl -fsS http://127.0.0.1:9091/api/v1/targets
curl -fsS http://127.0.0.1:9091/api/v1/rules
curl -fsS http://127.0.0.1:9094/-/ready
curl -fsS http://127.0.0.1:9094/api/v2/status
```

All expected targets must be `up`, the collector timestamp must advance, and
the alert rules must appear. Then exercise a harmless, labelled synthetic rule
under change control and prove its full lifecycle reaches Alertmanager, the
external provider, and the owned human escalation destination, including a
resolved notification. Record time-to-delivery without storing the webhook.

## Initial SLO and recovery objectives

These are proposed engineering indicators, not a business-approved SLA:

| Area | Initial objective / evidence | Important limitation |
| --- | --- | --- |
| API availability | 99.9% successful readiness probes over 30 days | Internal Compose probe; public HTTPS is a second signal, not an independent region |
| API server errors | Less than 1% 5xx responses over 30 days | Process-local counters reset and one low-traffic instance can distort short windows |
| API latency | p95 below 1.5 s for measured non-probe HTTP requests | Protocol upgrades have unbounded session length and are excluded; no route labels; client/network latency outside the API is excluded |
| Backup RPO | At most 24h15m scheduled interval; alert at 30 h | Only true while the timer/jobs/off-site provider succeed; no PostgreSQL PITR |
| Recovery RTO | Not measured or approved | Restore reports exclude host, DNS, secrets, download, reconciliation, smoke, and cutover |

Prometheus retains up to 35 days but the 5 GB size cap can shorten history.
Check actual oldest-sample age before producing a monthly SLO report. Formal
multi-window error-budget alerts and business approval remain follow-up work.
Backup and restore evidence is defined in `scripts/backup/README.md`; repeat a
timed isolated-host drill quarterly and after storage/schema/encryption changes.

## Incident workflow

1. Acknowledge and record the alert name, start time, affected service, and
   current metric value. If no notification provider exists, the on-duty
   operator must inspect Prometheus on an explicitly scheduled cadence.
2. Check whether the signal itself is healthy: Prometheus target state,
   collector timestamp, monitoring container health, and host time.
3. Preserve evidence before changing state: relevant journal output, bounded
   Compose logs, container inspect output, disk state, and recent deploy ID.
4. Assign incident command and communications ownership. Escalate a critical
   customer-facing or backup alert immediately under the organization's real
   policy; this repository does not define phone numbers or people.
5. Apply restarts, rollbacks, cleanup, certificate renewal, or traffic changes
   only after an authorized operator reviews blast radius and rollback.
6. Verify `/healthz`, `/readyz`, public HTTPS, core application smoke, and all
   affected alerts. Record recovery time and follow-up controls.

Useful read-only evidence commands:

```bash
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/app.env ps
docker compose -f deploy/docker-compose.prod.yml --env-file deploy/app.env \
  logs --since 30m --tail 500 api web
systemctl status drivergo-backup.timer drivergo-backup.service
journalctl -u drivergo-backup.service --since yesterday
df -h / /var/backups/drivergo/full
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/readyz
```

## PrometheusScrapeTargetDown

Open Prometheus `/targets` and distinguish the API, node exporter, and
blackbox-exporter scrape from the probed endpoint. Inspect monitoring Compose
state and recent logs. A blackbox scrape being down means the exporter is
unreachable; `probe_success=0` means the target it checked failed.

## APIHealthProbeFailed

Check the API container state, `/healthz`, recent API logs, OOM state, and the
last deployment. Liveness should not depend on PostgreSQL/Redis/MinIO; if it
fails while the process runs, preserve a goroutine/profile or crash signal
before an approved restart.

## APIReadinessProbeFailed

Query `/readyz` directly and inspect which dependency is reported unavailable.
Check PostgreSQL, Redis, and MinIO health and connection errors. Do not turn
readiness into a liveness restart loop; repair the dependency or roll back the
causal change under incident control.

## WebProbeFailed

Check web container health/logs, internal DNS, its API dependency, and the
landing page response. Compare the deployed image reference with the approved
release before rollback or restart.

## PublicTLSProbeFailed

Compare the internal health probe with the public URL. Inspect DNS resolution,
reverse-proxy logs/configuration, certificate validation, and upstream state.
A healthy internal probe plus failed public probe points to DNS, proxy,
firewall, CDN, or TLS rather than the API process.

## APIMetricsContractMissing

Fetch the API `/metrics` endpoint from the Prometheus network and confirm that
the request total, 5xx status-class counter, and duration histogram count are
present as Prometheus text. This commonly means the monitoring bundle was
deployed before the matching API revision or that content negotiation returned
the JSON compatibility view. Do not silence latency/error alerts until the
instrumentation contract is restored.

## APIHighErrorRatio

Confirm request volume and the absolute 5xx rate; low traffic is gated but can
still be noisy. Correlate the onset with deployment, dependency failures, and
API JSON logs. Identify the failing operation before any rollback. Metrics do
not expose route labels to avoid cardinality and privacy risk.

## APIHighP95Latency

Compare CPU/memory pressure, dependency latency, disk saturation, and request
volume. The histogram is end-to-end inside the Go handler and excludes probes
and WebSocket upgrades so session lifetime does not corrupt HTTP latency, but
has no route label. Use sampled logs/profiles in an approved diagnostic window
to localize the slow path.

## OpsCollectorMetricsMissing

Check the node-exporter target and confirm that
`/var/lib/drivergo-monitoring/textfile/drivergo_ops.prom` exists, is regular,
and is readable. Then inspect the collector timer and journal. Do not weaken
directory ownership or mount the Docker socket into node exporter as a fix.

## OpsCollectorFailed

Inspect the collector journal for the exact failed source. Common causes are
Docker/systemd unavailability, a moved Compose/env path, an unreadable
certificate, or a malformed Docker inspect response. The output is published
with `drivergo_ops_collector_success 0` before the oneshot exits non-zero.

## OpsCollectorStale

Check timer `LastTriggerUSec`, the service result, lock contention, system
clock, and file modification time. The collector uses a non-blocking lock and
an atomic rename, so a stuck overlapping run should fail rather than corrupt
the textfile. The rule compares absolute clock difference, so a host clock more
than three minutes ahead also fires instead of making stale data look fresh.

## HostDiskUsageHigh

Identify growth with read-only filesystem/container-log inspection. Check
backup sizes, Docker JSON logs, images, database growth, and Prometheus volume
retention. Schedule cleanup only for an explicitly reviewed target; never run
broad recursive deletion or remove the only good backup.

## HostDiskUsageCritical

Treat as imminent write/backup failure. Stop optional writers if authorized,
preserve the newest verified backup, and free only confirmed disposable data.
After recovery, verify databases, MinIO, backup service, and container health.

## MonitoredDiskPathMissing

For `target="backup"`, verify the configured root, filesystem mount, ownership,
and backup unit path. Recreating an empty directory will silence path absence
but must not be mistaken for recovered snapshots.

## ContainerNotRunning

Inspect Compose state and the container's exit code/OOM flag. Check dependency
ordering and the deployed image. Restart or roll back only after preserving
logs and confirming that stateful service recovery is safe.

## ContainerUnhealthy

Inspect the specific Docker health-check output and run the equivalent probe
read-only. A `starting` state during deploy should resolve within the five
minute `for` window. Repeated failure needs dependency/log investigation.

## ContainerRestarting

Inspect restart count, exit status, OOM state, resource limits, and logs across
all restarts. Counter resets after container recreation are expected; a fresh
container ID begins a new series history.

## BackupTimerInactive

Inspect timer load/active state and `systemctl list-timers`. Confirm that the
installed unit matches the reviewed repository unit. Enabling it is a live
mutation and requires operator approval; after doing so, verify an actual
encrypted, off-site-checked production run.

## BackupLastAttemptFailed

Read the backup unit journal from the first error backward. Check disk,
PostgreSQL/Redis/MinIO/Humo capture, age recipient, rclone provider access, and
checksum verification. Production backup fails if verified off-site upload
fails, so service success is also the repository's off-site-completion signal.
Do not weaken encryption or `REQUIRE_OFFSITE_BACKUP` to clear the alert.

## BackupSnapshotMissing

Inspect only exact `drivergo-YYYYMMDDTHHMMSSZ` directories. A snapshot counts
only after `scripts/backup/verify_snapshot.sh` validates its manifest, declared
sizes, exact file inventory, and SHA-256 payloads. Retrieve from the verified
off-site location if local media was lost.

## BackupSnapshotInvalid

Run `scripts/backup/verify_snapshot.sh` against each exact snapshot directory
under the protected backup root. Do not edit the failed snapshot or let it
count toward the retention minimum. The retention job moves invalid local
snapshots to the sibling `quarantine` directory without deleting them. Preserve
that evidence, verify the newest off-site copies, and create a new encrypted
backup before investigating storage, transfer, or disk-integrity failures.

## BackupSnapshotStale

Compare manifest `created_at`, service history, timer state, disk space, and
off-site provider health. The 30-hour threshold includes margin above the
committed maximum 24h15m schedule, not a guarantee. After repair, run and
verify a new production backup under change control.

## PublicTLSCertificateExpiring

Confirm the certificate actually served on the public hostname, issuer,
renewal automation, and DNS/proxy path. Renew through the authorized provider,
then validate the served chain and expiry from outside the origin network.

## OriginTLSCertificateMissingOrInvalid

This fires only when `TLS_CERT_FILE` is configured. Verify the absolute path,
symlink target, permissions available to the root oneshot, parseability, and
current validity interval. This local-file check does not validate issuer trust
or prove that the proxy serves this file. If origin-cert monitoring is
intentionally not applicable, remove that optional setting through the normal
configuration process.

## OriginTLSCertificateExpiring

Renew the configured local origin certificate using the approved issuer and
reload the terminating proxy through a reviewed change. Verify both the local
file and the certificate actually served on the intended connection path.

## Post-incident evidence

Record detection source, whether a human was actually notified, timeline,
customer impact, root cause, recovery actions, SLO/RPO/RTO impact, and concrete
owners/dates for prevention. Preserve a full restore-drill report for backup
incidents. If detection relied on manual Prometheus inspection, track alert
delivery as an unresolved critical operational dependency.
