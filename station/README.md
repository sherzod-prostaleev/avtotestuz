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

## What this does not do yet

This is the Faza 1 agent. It has no offline lease, no content cache, no
offline result queue, no clock-rollback protection, and no auto-update — the
agent needs a live connection to the backend to renew its token, and if the
backend is unreachable the proxy fails closed rather than degrading silently.
There is also no MSI/GPO installer yet; deployment today is copying the
binary and running it once per PC. Those are planned for Faza 2 and Faza 3.
