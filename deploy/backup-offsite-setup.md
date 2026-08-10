# Provider-backed encrypted off-site backup setup

This runbook closes P1-A only after a real provider remote, a completed remote
backup, and a restore drill are evidenced. Do not create provider accounts or
place credentials in this repository. The existing home-PC pull remains an
interim second-host copy, not provider-backed off-site storage.

Run these commands on the VPS as an operator with `sudo`. Keep terminal output
free of credentials; do not use `set -x`, `env`, or `rclone config show`.

## 1. Choose a dedicated provider prefix

Choose one supported rclone backend: Cloudflare R2 (S3), Backblaze B2, another
S3-compatible provider, or SFTP. Provision the account, bucket/directory, and
least-privilege credentials outside Git. The remote must use a dedicated,
non-root prefix, for example:

```text
drivergo-offsite:drivergo/full
```

Do not reuse the VPS filesystem, a broad bucket root, or a prefix shared with
unrelated data. Configure retention/object-lock policy with the provider
separately; the backup scripts do not claim immutability supplied by a provider.

## 2. Create the protected rclone configuration

Create the configuration directory and use rclone's interactive setup. Select
the provider backend and enter the provider credentials only in that protected
host file:

```bash
sudo install -d -o root -g root -m 0700 /etc/drivergo
sudo rclone config --config /etc/drivergo/rclone.conf
sudo chown root:root /etc/drivergo/rclone.conf
sudo chmod 0600 /etc/drivergo/rclone.conf
sudo stat -c '%U:%G %a %n' /etc/drivergo/rclone.conf
```

The final command must report `root:root 600 /etc/drivergo/rclone.conf`. Never
commit or copy this file into an image, support bundle, chat, or shell history.

## 3. Extend the protected backup environment

Edit `/etc/drivergo/backup.env` using `sudoedit`. Preserve the existing public
`AGE_RECIPIENT` and add the following values, replacing only the remote name
and dedicated prefix:

```dotenv
RCLONE_CONFIG=/etc/drivergo/rclone.conf
RCLONE_REMOTE=drivergo-offsite:drivergo/full
```

The file must remain root-owned and mode 0600. The age private identity stays
on the operator PC; it must not be copied to the VPS.

```bash
sudo chown root:root /etc/drivergo/backup.env
sudo chmod 0600 /etc/drivergo/backup.env
sudo stat -c '%U:%G %a %n' /etc/drivergo/backup.env
```

## 4. Run dry and live preflight checks

Load the protected environment only into a root shell, without printing it.
The dry check confirms recipient presence, remote syntax, and `rclone.conf`
permissions without contacting the provider:

```bash
sudo -i
set -a
. /etc/drivergo/backup.env
set +a
OFFSITE_PREFLIGHT_SKIP_REMOTE=1 /opt/drivergo/scripts/backup/offsite_preflight.sh
```

Then run the live read-only remote listing. It succeeds only if rclone can
reach the configured dedicated prefix:

```bash
/opt/drivergo/scripts/backup/offsite_preflight.sh
exit
```

The preflight intentionally prints neither the age recipient nor remote
configuration. Resolve any failure before proceeding.

## 5. Produce and validate a provider-backed snapshot

Keep the interim timer enabled until the following evidence is complete. Start
the production service once to run its committed `--production` policy:

```bash
sudo systemctl daemon-reload
sudo systemctl start drivergo-backup.service
sudo journalctl -u drivergo-backup.service -b --no-pager
```

Record the new snapshot ID from the journal. Fetch only its completion evidence
to a secure operator work directory, then verify that the remote marker binds
that snapshot to its checksum manifest:

```bash
sudo -i
set -a
. /etc/drivergo/backup.env
set +a
snapshot=drivergo-YYYYMMDDTHHMMSSZ
workdir="$(mktemp -d)"
rclone copyto "${RCLONE_REMOTE%/}/${snapshot}/REMOTE_COMPLETE" "$workdir/REMOTE_COMPLETE"
rclone copyto "${RCLONE_REMOTE%/}/${snapshot}/SHA256SUMS" "$workdir/SHA256SUMS"
sha256sum -- "$workdir/SHA256SUMS"
cat "$workdir/REMOTE_COMPLETE"
rm -rf -- "$workdir"
exit
```

The `sha256sums_sha256` value in `REMOTE_COMPLETE` must equal the displayed
SHA-256 of `SHA256SUMS`. Also use `rclone check --one-way --checksum` against a
fresh local copy when operationally feasible; do not treat a listing alone as
remote integrity evidence.

## 6. Run a restore-from-remote drill

On an isolated drill host or guarded scratch location, download the entire
completed remote snapshot, verify it, then run the existing full drill. Do not
restore into the live application services:

```bash
sudo -i
set -a
. /etc/drivergo/backup.env
set +a
snapshot=drivergo-YYYYMMDDTHHMMSSZ
drill_root=/tmp/drivergo-restore-drill
download_root=/var/tmp/drivergo-remote-drill
install -d -m 0700 "$download_root" "$drill_root"
rclone copy "${RCLONE_REMOTE%/}/${snapshot}" "$download_root/$snapshot" --checksum
/opt/drivergo/scripts/backup/verify_snapshot.sh "$download_root/$snapshot"
RESTORE_DRILL_ACK=avtotest_restore_drill \
  DRILL_ROOT="$drill_root" \
  /opt/drivergo/scripts/backup/full_restore_drill.sh "$download_root/$snapshot"
exit
```

Retain the generated drill evidence and clean the downloaded encrypted
snapshot/scratch only under the established retention procedure.

## 7. Switch scheduling after evidence passes

Only after the live preflight, `REMOTE_COMPLETE` validation, and remote restore
drill all pass, switch from the interim timer. The production service already
executes `backup_all.sh --production`; never pass an alternate policy or enable
both schedules.

```bash
sudo systemctl disable --now drivergo-backup-homepc.timer
sudo systemctl enable --now drivergo-backup.timer
sudo systemctl start drivergo-backup.service
sudo systemctl list-timers drivergo-backup.timer drivergo-backup-homepc.timer
sudo journalctl -u drivergo-backup.service -b --no-pager
```

Confirm `drivergo-backup.timer` is active, the home-PC timer is inactive, and
observe at least one scheduled success before claiming automatic
provider-backed off-site backup. The operator PC may remain an additional
recovery copy.
