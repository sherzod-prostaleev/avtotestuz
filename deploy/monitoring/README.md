# Driver Go production monitoring bundle

This directory is a repository-side, opt-in monitoring layer. It does not
contact or change the live VPS by itself.

The design deliberately keeps Docker control-plane access out of long-lived
containers:

- Prometheus scrapes the API's `/metrics`, a textfile-only node exporter, and
  blackbox probes for API liveness/readiness, web, and public HTTPS.
- A root-owned systemd oneshot reads an exact Compose service allowlist,
  backup unit/snapshot state, disk capacity, and an optional local certificate.
  It atomically publishes metrics once per minute and then exits.
- Node exporter mounts only `/var/lib/drivergo-monitoring/textfile` read-only.
  It does not mount `/`, `/proc`, `/sys`, or `docker.sock`.
- Images are not assigned floating tags in Git. The environment validator
  requires operator-supplied `registry/repository@sha256:<digest>` references.
- Prometheus is loopback-only, has bounded 35-day/5 GB retention, and contains
  alert evaluation rules.
- Alertmanager is loopback-only and routes firing/resolved alerts to one
  operator-owned HTTPS webhook read from a root-owned, dedicated-group
  mode-0640 file. Image digests and
  that protected receiver file are mandatory, so deployment fails closed.
  Until a real receiver and synthetic end-to-end page are verified, alert
  delivery/paging is not operational.

Files:

- `docker-compose.monitoring.yml`: isolated, hardened monitoring services.
- `prometheus.yml`, `blackbox.yml`, `alerts.yml`, `alertmanager.yml`:
  scrape/probe/rule and delivery-routing contract.
- `write_textfile_metrics.sh`: bounded host collector with locking and atomic
  output.
- `systemd/`: oneshot and one-minute timer.
- `validate_env.sh`: non-evaluating immutable image-reference validation.
- `RUNBOOK.md`: install, validation, incident triage, SLO/RPO/RTO limits.
- `tests/monitoring_contract_test.sh`: offline/static contract and mocked
  collector tests.

Run the repository-only check from the project root:

```bash
deploy/monitoring/tests/monitoring_contract_test.sh
```

The main CI operations-contract job runs this check on every change.
