# Windows 7 release smoke-test checklist

This checklist produces a reviewable evidence bundle for a real Windows 7
release test. It is **not** evidence that Windows 7, Go 1.20, or this station
client is secure long-term. Run it for every station release and keep the
bundle with that release record.

Run the steps on two separate Windows 7 SP1 machines (PC A and PC B). Use a
non-production test organisation and test installer key only. Never copy a
production `station.key`, installer key, access token, certificate, or private
key into the evidence bundle.

## Prerequisites

1. Obtain the exact `avtotest-station.exe` release candidate and record its
   SHA-256 in the release ticket. Do not rebuild it on the test PC.
2. Confirm both PCs are Windows 7 SP1 and record their hostnames in the
   release ticket. The station is built as a Windows/386 binary, so test the
   oldest supported 32-bit environment when it is available.
3. On PC A, create a writable bundle directory:

   ```bat
   mkdir "%USERPROFILE%\Desktop\avtotest-win7-evidence"
   ```

4. Use `cmd.exe` for the commands below. Do not put a test installer key in a
   redirected output file, a screenshot, or `manifest.env`.

## 1. Self-test, DPAPI, registry, and autostart

On PC A, in the directory containing the exact release candidate:

```bat
avtotest-station.exe -selftest > "%USERPROFILE%\Desktop\avtotest-win7-evidence\selftest.txt" 2>&1
echo %ERRORLEVEL%
```

The exit code must be `0`; `selftest.txt` must contain all seven `[n/7]` PASS
lines and `SELFTEST RESULT: PASS`. These checks cover the registry-derived
hardware ID, DPAPI seal/unseal, ciphertext-not-plaintext, tamper rejection,
empty-file rejection, HKCU autostart round-trip, and install-target write.

Capture the Windows registry value independently:

```bat
reg query "HKLM\SOFTWARE\Microsoft\Cryptography" /v MachineGuid > "%USERPROFILE%\Desktop\avtotest-win7-evidence\registry.txt" 2>&1
```

`registry.txt` must show the queried key, `MachineGuid`, and a non-empty
`REG_SZ` value. A failed query is a failed smoke test.

## 2. Cross-machine DPAPI binding

PC A must have a station enrolled only to the non-production test organisation.
Copy its `%ProgramData%\AvtoTest\station\station.key` to removable media. Do
not place the key in the evidence directory and securely delete the removable
copy after this test.

On PC B, copy that file to a temporary path, then run:

```bat
avtotest-station.exe -selftest-import C:\Temp\station.key > C:\Temp\dpapi-cross-machine.txt 2>&1
echo %ERRORLEVEL%
```

The command must exit `0` and print
`RESULT: correctly bound to its original machine`. An exit of `1` or
`RESULT: SECURITY FAILURE` is a release blocker. Copy only the text output
(never `station.key`) into PC A's bundle as `dpapi-cross-machine.txt`, then
prepend these two lines with the actual distinct machine names:

```text
source_machine=PC-A-HOSTNAME
destination_machine=PC-B-HOSTNAME
```

## 3. Owned HTTPS through the station path

Use the non-production test installer and an allowed test station on PC A.
Configure it only with the owned release origin `https://drivergo.uz`; do not
disable certificate validation, install a local CA, or use an IP address.
Start the station with `-no-kiosk`, then request its local kiosk endpoint from
the same PC and complete one normal station-backed request using the browser.
Verify in the controlled backend request log that the request came through the
station proxy and reached the owned HTTPS origin with certificate validation
enabled. Do not copy request headers, tokens, or installer codes into evidence.

Create `owned-https.txt` in the bundle with these exact completed fields (the
HTTP status must be a successful 2xx status):

```text
origin=https://drivergo.uz
tls_certificate_validation=PASS
http_status=200
station_proxy=PASS
```

`station_proxy=PASS` is permitted only after the local kiosk and controlled
backend log both prove the request used the station path. A direct browser
request to the origin is insufficient.

## 4. Write the manifest and validate offline

Create `manifest.env` in the bundle, replacing only the non-example values:

```text
format=avtotest-win7-smoke-v1
station_version=1.0.0
operator=release-operator
tested_at_utc=2026-08-10T06:00:00Z
owned_origin=https://drivergo.uz
```

## 5. Authenticode release gate

Authenticode signing is required before distribution. This repository does not
sign binaries and must never contain a signing certificate or private key.
After the functional smoke steps, an authorised release operator must sign the
exact release artifact in the organisation's managed signing service and
verify both signature and timestamp on a Windows machine. Record only the
result, not certificate material, in `authenticode.txt`:

```text
signature_verification=PASS
timestamp_verification=PASS
```

Failure to sign or verify is a release blocker.

## 6. Validate and archive

On a trusted Linux workstation or CI runner, validate the bundle without a VM,
network access, or secrets:

```sh
backend/station/security/run_win7_smoke.sh /path/to/avtotest-win7-evidence
```

The validator fails closed on a missing, empty, malformed, unapproved-origin,
or failed evidence file. Archive the validated bundle with the release ticket.
