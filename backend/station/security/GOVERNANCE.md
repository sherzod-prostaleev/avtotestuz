# Windows 7 station vulnerability exception governance

The Go 1.20/Windows 7 station binary is an explicit, time-bounded
compatibility exception. Windows 7 and Go 1.20 are end of life. This document
does not represent a long-term security claim.

## Scope and ownership

Every ID in `win7-govuln-allowlist.txt` is covered by this record:

- **Owner:** Sherzod, station maintainer
- **Last review:** 2026-08-10
- **Next review:** 2026-11-10, and before every station release
- **Evidence:** the CI `station-vulnerability-scan` job retains the exact
  Windows/386 binary build metadata and `govulncheck -mode=binary` report.

The owner must remove a resolved ID rather than renewing it. Any new ID must
be reviewed before it is added to the allowlist and inherits this record only
after that review records its affected component and mitigations in the pull
request.

## Current exception classes and exit conditions

| IDs | Why retained | Exit condition |
| --- | --- | --- |
| All `GO-*` IDs other than `GO-2026-5024` | They are reachable findings in Go 1.20.14's end-of-life standard library. Their fixed releases require leaving the Go 1.20 family. | Retire Windows 7 and rebuild the client on a maintained Go release, or provide a separately supported Win7-compatible client whose binary scan no longer reports the ID. Until then, every release must also have a passing real-machine evidence bundle validated by `security/run_win7_smoke.sh`; this is a compatibility gate, not a security exit. |
| `GO-2026-5024` | The station uses `golang.org/x/sys/windows` for DPAPI and registry access. The fixed version is `v0.44.0`; it cannot compile with Go 1.20. The highest tested compatible version is `v0.30.0`, which remains affected. | Adopt an `x/sys` release containing the fix that builds and passes the Windows/386 tests under Go 1.20, or meet the standard-library exit condition above. Until then, retain a passing real-machine evidence bundle validated by `security/run_win7_smoke.sh`; it does not remove this vulnerability exception. |

## Required release evidence

Before release, build exactly with `CGO_ENABLED=0 GOOS=windows GOARCH=386`
using Go 1.20, then retain the binary-mode `govulncheck` report. Run the real
smoke test on a Windows 7 VM or machine: execute `-selftest`, verify DPAPI
sealing/unsealing and registry hardware-ID behavior, verify cross-machine
DPAPI rejection, and run the station's owned-HTTPS connection path. Follow
`win7_smoke_checklist.md` exactly and archive the resulting evidence bundle
only after `security/run_win7_smoke.sh EVIDENCE_DIRECTORY` accepts it. Linux
tests and cross-compilation can validate the harness contract, but cannot prove
the Windows behavior.

Authenticode signing and reputation work remain required before any claim of
long-term operational compatibility. This repository does not perform live
signing and must never contain a certificate or private key.
