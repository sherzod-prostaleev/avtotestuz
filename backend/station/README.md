# avtotest-station

`avtotest-station` is the program that runs on a classroom PC. It holds the
station's Ed25519 key, keeps a 15-minute access token live against the
AvtoTest backend, serves the kiosk browser from `127.0.0.1`, and opens Chrome
in kiosk mode pointed at that local address. The browser never sees the
station's key or its access token — only the agent does.

## Install (IT staff, one line)

Copy `avtotest-station.exe` to the classroom PC and run it once with the
enrollment code the teacher generated in the school dashboard:

```
avtotest-station.exe -code AVTO-XXXX-XXXX
```

That first run binds this specific machine to the school's org, draws one
seat from the school's licence, and writes the station's key and state to
disk. Every run after that needs no flags — it already knows who it is:

```
avtotest-station.exe
```

Add `-label "Kabinet 3, PC-7"` on first run to give the station a name the
school recognizes in its station list; it defaults to the machine's hostname.
Add `-no-kiosk` to run the local server without launching a browser, useful
for testing the agent on a machine that has no display.

## Where the key and state live

By default: `%ProgramData%\AvtoTest\station\`. That directory holds:

- `station.key` — the sealed Ed25519 private key
- `station.json` — the station id, org id and label returned at enrollment

`%ProgramData%` is used, not a user profile, because the agent runs as a
machine-wide service and no operator is necessarily logged in.

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

1. Run `avtotest-station.exe -code AVTO-XXXX-XXXX` with a fresh enrollment
   code from the school dashboard.
2. The agent generates a new key, enrolls as a new station, and draws a seat.
3. The school (or support) revokes the old station entry so its seat is
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
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -X main.version=1.0.0" -o avtotest-station.exe ./cmd/avtotest-station
```

- `-X main.version=1.0.0` stamps the version `main.go` logs on startup and
  prints in `-selftest`; bump it per release.
- `-s -w` strips debug symbols and the DWARF table — smaller binary, no
  effect on behavior.
- The result is one `.exe` (around 7 MB as of this writing). Copying that
  file to `%ProgramData%` or anywhere else, and running it, is the entire
  install — see "Install (IT staff, one line)" above.

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
prints `SELFTEST RESULT: PASS` and all five lines say `PASS`.** Running the
same command on a Linux dev build is expected to print `FAIL` on check 3 (and
usually 4) — that build stores the key in the clear on purpose, so seeing it
fail there is confirmation the check is real, not a rubber stamp.

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

## What this does not do yet

This is the Faza 1 agent. It has no offline lease, no content cache, no
offline result queue, no clock-rollback protection, and no auto-update — the
agent needs a live connection to the backend to renew its token, and if the
backend is unreachable the proxy fails closed rather than degrading silently.
There is also no MSI/GPO installer yet; deployment today is copying the
binary and running it once per PC. Those are planned for Faza 2 and Faza 3.
