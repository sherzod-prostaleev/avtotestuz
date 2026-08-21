# avtotest-station

`avtotest-station` is the program that runs on a classroom PC. It holds the
station's Ed25519 key, keeps a 15-minute access token live against the
AvtoTest backend, serves the kiosk browser from `127.0.0.1`, and opens Chrome
in kiosk mode pointed at that local address. The browser never sees the
station's key or its access token — only the agent does. The kiosk itself
has no login: a learner is never asked for an account, because VIP comes
from the station's own binding to the school's licence, not from anyone
signing in.

This module lives at `backend/station/` (a separate Go module,
`avtotest.uz/station`) and is built and shipped as part of the API image;
see `make station-check` at the repo root for its own lint/test/build target.

## Install

### The normal way: the downloaded installer

The school's admin opens the org's **installer key** in the admin panel and
downloads `avtotest-station-<slug>.exe`. That download is a plain
`avtotest-station.exe` with the org's enrollment code appended to its own
tail (see `backend/internal/b2b/installer.go` for how the trailer is written
and `internal/embedcfg` for how this binary reads it back). Nothing is typed
on the classroom PC. Copy that one file to it and run it once:

```
avtotest-station.exe
```

That first run reads its own embedded code, binds this machine to the
school's org, draws one seat from the licence, copies itself into
`%ProgramData%\AvtoTest\station\`, registers a `HKCU` autostart entry (see
`internal/selfinstall`), and opens the kiosk. Every time after that — every
time the same Windows account logs back in, not merely every time the PC
boots, since the autostart entry lives under that account's
`HKCU\...\Run` — needs nothing typed at all; it already knows who it is and
comes back on its own. That only happens unattended if the classroom PC
auto-logs into that same account; see "Where the key and state live" below.

Downloading the same installer again later reuses the same key: it does not
disturb PCs already enrolled from earlier copies of the file, and the same
key can enroll every PC in the school up to its licensed seat count. If a
downloaded file leaks, the admin can **rotate** the key from the same panel
page — files already handed out stop being able to enroll new PCs, but
stations that already enrolled keep working untouched.

### The manual way: `-code`

A plain build with no embedded code (a local dev build, or a copy that
reached the PC by some route other than the admin panel download) can be
pointed at an org by hand with the same code shown on the installer key
panel:

```
avtotest-station.exe -code AVTO-XXXX-XXXX
```

The code is the org's live installer key, not a separate per-PC secret, so
the same one works for every PC in the school until it is rotated — this
first run does enroll the PC, draw a seat, and open the kiosk, the same as
the downloaded installer.

Where it does **not** match the downloaded installer: `selfinstall.Ensure`
only runs when the binary has an embedded code baked into its own tail (see
`internal/selfinstall`'s package doc), and a manually-passed `-code` flag
never sets that. So this run does not copy itself into
`%ProgramData%\AvtoTest\station\` and does not register an autostart entry —
the agent keeps running from wherever it was launched, and it will **not**
come back after the next reboot. Use this path for testing the agent, or for
a one-off recovery run, not for putting a classroom PC into service. For a
real classroom PC, use the downloaded installer above.

Add `-label "Kabinet 3, PC-7"` on first run to give the station a name the
school recognizes in its station list; it defaults to the machine's hostname.
Add `-no-kiosk` to run the local server without launching a browser, useful
for testing the agent on a machine that has no display.
Add `-uninstall` to remove the installed copy and its autostart entry, and
delete this station's local key and state:

```
avtotest-station.exe -uninstall
```

This only touches the local PC — it does not free the station's seat.
Revoke the station in the admin panel too, or the licence keeps holding
that seat.

## Where the key and state live

By default: `%ProgramData%\AvtoTest\station\`. That directory holds:

- `station.key` — the sealed Ed25519 private key
- `station.json` — the station id, org id and label returned at enrollment

`%ProgramData%` is used, not a user profile, so the install directory and
seal don't move if a different account happens to touch the machine — but
that does **not** mean the agent runs machine-wide or without a logged-in
operator. Autostart is a `HKCU\...\Run` entry (see "The normal way" above),
which only fires for the one Windows account that ran the installer. The
classroom PC must be configured to **auto-login to that same account**: a
second Windows account on the same machine will not launch the kiosk on its
own, because the Run entry simply is not registered for it. Treat "one PC,
one Windows account, that account auto-logs in" as a hard requirement of the
install, not an incidental detail.

## The key is DPAPI machine-scoped

On Windows, `station.key` is sealed with `CRYPTPROTECT_LOCAL_MACHINE` DPAPI.
The ciphertext is tied to that specific Windows installation: copying
`station.key` to another PC, even one belonging to the same school, produces
a file that PC cannot decrypt. There is no way to "restore" a station's
identity onto different hardware — a station is a (key, hardware id) pair,
and both halves must be present on the machine that enrolled.

## Re-imaging a PC

Re-imaging or replacing a classroom PC invalidates its old key beyond
recovery — that is the point of DPAPI machine-scoping, not a bug to route
around. To bring the PC back:

1. Run the same downloaded installer again on the re-imaged PC (see "The
   normal way" above). Use the manual `-code AVTO-XXXX-XXXX` form only for a
   one-off test of the re-imaged machine — it does not self-install or
   register autostart, so it will not survive the PC's next reboot and is
   not a substitute for the installer when actually returning the PC to
   service.
2. The agent generates a new key, enrolls as a new station, and draws a
   seat — the installer key is not consumed by one PC, so reusing it here is
   expected, not a special case.
3. The admin revokes the old station entry in the admin panel so its seat is
   returned to the pool; the licence enforces a seat cap, not a device count,
   so nothing is lost by re-enrolling.

## Non-Windows builds are development-only

This module builds on Linux and macOS so the proxy and agent logic can be
tested without Windows. On those platforms `keystore` writes the private key
to a plain `0600` file with **no sealing at all** — see
`internal/keystore/keystore_other.go`. That build must never be shipped to a
school: without DPAPI, the key file is just a file, and copying it copies the
station's identity. The Windows build (`GOOS=windows`) is the only one with
real protection.

## Building a release binary

The agent is a single, statically-linked Windows executable with no
installer and no runtime dependency (no .NET, no VC++ redistributable, no Go
install on the target machine). Build it from `backend/station/` with:

```
CGO_ENABLED=0 GOOS=windows GOARCH=386 go build \
  -ldflags "-s -w -H windowsgui -X main.version=$(cat VERSION)" \
  -o avtotest-station.exe ./cmd/avtotest-station
```

In production this is done by `backend/Dockerfile`'s `station` stage, which
reads the same `VERSION` file. Nothing has to be typed at build time; see
"Versioning" below.

- Go 1.20 is intentional: it is the final Go family supporting Windows 7.
  `GOARCH=386` keeps the same executable usable on 32-bit and 64-bit Windows
  7 machines. Do not raise this module/toolchain independently of the Win7 VM
  compatibility gate.

### Security exception for Windows 7

Go 1.20 is end-of-life and its standard library has published high/critical
advisories that cannot be patched without moving to a Go family that drops
Windows 7. This is a deliberate compatibility exception, not a clean security
scan. CI therefore does both of the following:

- hard-fails `govulncheck` when a known vulnerable symbol is reachable from
  the station module; and
- uploads the complete Trivy high/critical report for the embedded `.exe`
  while the maintained API/web/Humo runtime images remain hard-gated at zero.

Keep station traffic outbound to the owned HTTPS origin, the local listener on
`127.0.0.1`, response/time limits bounded, and the station module isolated from
the server toolchain. Review this exception at every station release and at
least quarterly. The long-term exit is retiring Windows 7 (or supplying a
separately supported compatibility client), then rebuilding on a maintained Go
release; silently raising `go 1.20` is not acceptable because it strands the
installed classroom fleet.
- `-X main.version=...` stamps the version the agent logs on startup, reports
  to the backend as `b2b_station.agent_version`, shows on the kiosk status
  screen, and compares against the update manifest. It comes from
  `backend/station/VERSION` — see "Versioning" below.
- `-H windowsgui` links the binary as a GUI application, which is what stops a
  classroom PC from showing a black console window. It costs a console for the
  diagnostic modes, which `cmd/avtotest-station/console_windows.go` borrows
  back with `AttachConsole` when the operator runs the agent from `cmd`.
- `-s -w` strips debug symbols and the DWARF table — smaller binary, no
  effect on behavior.
- The result is one `.exe` (around 7 MB as of this writing). This is the
  binary the admin panel serves for download and appends each org's config
  to (see "Install" above) — copying it to `%ProgramData%` or anywhere else,
  and running it, is the entire install.

## Verifying the build actually works on Windows (`-selftest`)

Everything about the key sealing (`internal/keystore/keystore_windows.go`,
DPAPI) and the hardware id (`internal/hwid/hwid_windows.go`, the registry
`MachineGuid`) only runs on Windows. Development happens on Linux, where
both are replaced with plain, unsealed stand-ins on purpose (see
"Non-Windows builds are development-only" below) — so the only way to know
the real thing works is to run it on a real Windows PC. `-selftest` does
that in one command, with no Go toolchain required:

```
avtotest-station.exe -selftest
```

This does **not** touch the real `%ProgramData%\AvtoTest\station\`
enrollment — it works entirely inside a temporary directory it deletes when
it finishes, so it is safe to run on a classroom PC that already has a
working station on it. It checks, in order:

1. **Hardware id** — that `hwid.Collect()` returns a 64-character hex id.
2. **Seal round-trip** — that a key saved through the real keystore and
   loaded back is byte-for-byte the same key.
3. **The file is genuinely sealed** — that the raw bytes written to
   `station.key` do not contain the plaintext private key anywhere in them.
4. **Tamper rejection** — that flipping one byte of the sealed file makes
   `Load` fail cleanly instead of returning a wrong key or crashing.
5. **Empty file rejection** — that a zero-byte key file (an interrupted
   write, a full disk) also fails cleanly.

It prints one `PASS`/`FAIL` line per check, then a summary line, and exits
with a non-zero code if anything failed — so it can be scripted (e.g. an IT
rollout step that refuses to proceed on `FAIL`). **A passing run on Windows
prints `SELFTEST RESULT: PASS` and all seven lines say `PASS`.** Running the
same command on a Linux dev build is expected to print `FAIL` on check 3 (and
usually 4) — that build stores the key in the clear on purpose, so seeing it
fail there is confirmation the check is real, not a rubber stamp.

For a release, this single-machine check is insufficient. Follow
[`security/win7_smoke_checklist.md`](security/win7_smoke_checklist.md) on two
real Windows 7 machines, then validate the collected evidence bundle with
`security/run_win7_smoke.sh`. That compatibility gate does not make any
long-term Windows 7 security claim.

One property genuinely needs two machines and `-selftest` says so at the
end of its own output: that a key sealed on PC A cannot be unsealed on PC B.
To finish that check:

```
avtotest-station.exe -selftest-import <path-to-a-station.key-copied-from-another-PC>
```

Copy a real `station.key` from `%ProgramData%\AvtoTest\station\` on one
Windows PC onto a second Windows PC (USB drive, network share, anything —
it never writes back to the source path) and run the command above there.
Read the verdict as printed, not by whether the command "errored":

- If it prints **"RESULT: correctly bound to its original machine"**, the
  import failed to unseal — that is the *good*, expected outcome. The key
  is worthless off its original PC.
- If it prints **"RESULT: SECURITY FAILURE"**, the key unsealed on a
  machine it did not come from. That means the binding is broken; stop and
  report it — do not read this as a harmless pass.

## Versioning

`backend/station/VERSION` is the single source of truth. The Docker build reads
it, stamps it into both the binary and the PE version resource, writes it
beside the binary as `/station/agent-version` for the update manifest to serve,
and **fails the build** if it is missing or malformed.

It used to be a shell variable (`STATION_VERSION=1.0.9 docker compose build
api`) that an operator had to remember on every build. Nobody did, so from
2026-08-07 every deployed agent reported `1.0.0` while three different builds
were in the field, and `b2b_station.agent_version` was useless for telling a
school on a broken build from one on the fix. CI (`station-version-gate`) now
fails any pull request that changes a shipped file under `backend/station/`
without bumping `VERSION`.

## Self-update

An installed agent keeps itself current. Every six hours it reads
`GET /api/v1/b2b/stations/agent-manifest` — `{version, sha256, size}` — and
when the version differs from its own it:

1. downloads the plain, school-agnostic agent from
   `GET /api/v1/b2b/stations/agent?v=<version>`;
2. checks the SHA-256 against the manifest **before** touching anything;
3. appends the config trailer its own installed copy already carries
   (`internal/embedcfg.RawTrailer` — the school's installer key, which
   therefore never crosses the network again after the first install);
4. verifies the assembled file reads back as a configured agent;
5. renames the current binary to `avtotest-station.exe.prev` and moves the new
   one into place.

The running process is **not** killed mid-lesson. Windows refuses to overwrite
a running image but allows renaming it, so the swap is safe while the current
process keeps executing from the renamed file, and the new binary takes over at
the next start — which on a classroom PC is the next morning. The agent will
restart itself sooner, but only after the kiosk has made no API call for 30
minutes, which is longer than any exam.

Because any version difference triggers an update, reverting the API image is
also the kill switch: the fleet follows the image back down.

**PCs installed before 1.1.0 have no update client and will never pick this
up.** They need one last manual download from the admin panel, run once per
machine. After that they update themselves.

The integrity story is a SHA-256 over TLS, not a code signature: whoever can
serve the manifest can serve a matching binary. That closes truncated
downloads, stale edge caches and rewriting proxies, and it does not close a
compromised origin. Authenticode signing is the next step — see
`security/GOVERNANCE.md`.

## No console window

The shipped binary is linked `-H windowsgui`, so nothing appears on screen when
autostart runs it. Diagnosis moved to two places:

- `station.log`, beside the key in `%ProgramData%\AvtoTest\station\`
  (falling back to `%LOCALAPPDATA%\AvtoTest\station\` when that folder was
  created by a different Windows account and is not writable). It rotates at
  2 MB.
- the kiosk page itself, which polls `http://127.0.0.1:17817/__station/status`
  whenever it cannot reach the API and shows the agent's version, school,
  station id, log path and — in Uzbek — what is wrong and what to do about it.

A failure severe enough to stop the agent before it can serve anything raises a
Windows message box, because a GUI process printing to a stream nobody is
reading is a failure nobody sees.

## Offline is not supported

The classroom needs a live internet connection at all times. Every question,
image and answer is proxied from the AvtoTest backend in real time — nothing
is cached on the PC — so if the connection drops, the proxy fails closed and
the kiosk stops immediately rather than degrading or serving stale content.
Do not tell a school otherwise: there is no offline mode, no offline lease,
and no queued results to replay once connectivity returns.

## What this does not do yet

Beyond having no offline story at all (above), it has no clock-rollback
protection and no MSI/GPO installer; deployment today is downloading the binary
from the admin panel and running it once per PC, or copying it by hand. After
that first install the agent updates itself (see "Self-update").

It also cannot roll back a bad release on its own. `avtotest-station.exe.prev`
is kept so a person can restore one by hand, but an update that starts and then
misbehaves is caught by nobody, and an update that will not start at all leaves
no process able to react. The protections that exist are upstream: CI builds
and vets the exact Windows/386 artifact, the digest is checked before the swap,
and reverting the API image walks the whole fleet back.
