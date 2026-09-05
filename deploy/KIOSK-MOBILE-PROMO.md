# Kiosk mobile promotion (agent 1.3.1)

| Requirement | Implementation | Verification |
|---|---|---|
| REQ-001: attractive QR banner below the kiosk home content | MobilePromoBanner; three locales; no overlay over exams | Playwright at the three classroom resolutions (1366x768, 1280x800, 1024x768); first-screen screenshots |
| REQ-001b: the QR is on screen permanently, like an advert | Pinned MobilePromoStrip, fading out while the full banner is on screen; reserved space so it never covers the page end | Measured: the banner alone began at y=816 on 1366x768, 1280x800 and 1024x768 — below the fold on every classroom resolution. Playwright now asserts a >=64px QR fully inside the first screen at scrollY 0 |
| REQ-002: admin controls each school's URL and enabled flag | B2B school MobilePromoPanel; users.read / users.entitlements.grant; transactional audit | Admin HTTP integration and form tests |
| REQ-003: QR encodes the exact submitted URL | Local QR generation from original bytes; no redirects or normalization | Independent zbarimg decode, including query escaping and fragment |
| REQ-004: every kiosk sees only its own school's active promotion | Profile-bound station lookup; active school, station and licence required; disabled by default | Cross-school, disabled and suspended school integration tests |
| REQ-005: remote settings reach the classrooms | Asked once per mount of the station home screen (student returning to it, reload, or boot), 10 second timeout, nothing rendered on error. Deliberately no polling — see below | Unit test asserting exactly one request across 45 minutes of fake time, verified red by reintroducing an interval (42 calls) |
| REQ-006: release through existing kiosk updater | VERSION 1.3.1; background promo calls do not reset idle timer | station-check, Windows/386 build and updater tests; deployed manifest and fleet inspection |

## Operation

Admin → B2B → choose a school → “Mobil ilova uchun”. Enter the complete URL,
enable the switch and save. The saved banner is previewed in the same panel.
Disabling preserves the URL for later reuse. Other schools remain unchanged.
There is no platform-wide default: each school is set on its own page, and a
school with the switch off gets exactly the station screen it had before this
feature existed — no pinned strip, no banner, no reserved space (pinned by the
“switched off sees no trace of it” e2e case, which compares document heights).
Only HTTP(S) URLs without whitespace or embedded credentials are accepted,
up to 512 bytes. QR creation makes no request to the target URL or a third party.

## Rollout

Migration 0071 is additive: existing schools start disabled. Rebuild both API
(which carries the Windows agent) and web. Do not seed or reset data.
The frontend is served remotely: kiosks get the new home screen on the next
page load/navigation. An already-open old page needs a reload or next launch
before it can poll these new settings.

Agents with the existing updater check on startup and every six hours. They
stage the new executable and activate it at the next boot or after 30 minutes
without study activity. In 1.3.1, promotion polls do not delay that idle window.

The advert itself does not wait for the agent: the station agent proxies every
non-API request to the remote frontend, so a web deploy alone puts the banner
on every school's kiosks at the next page load. Pre-1.1.0 / manual-update /
offline PCs cannot be claimed updated merely because a release was deployed;
inspect reported agent_version per school.

## Why this screen must never poll

The first shipped version asked the API for the advert every ~65 seconds so
that an idle classroom lit up on its own when the switch was flipped. That is
exactly the wrong trade, and production proved it within the hour.

The updater installs a staged build only after the kiosk has made no proxied
API call for 30 minutes — that is how it knows the room is empty and nobody is
mid-exam. Agent 1.3.1 excludes this one path from that idle clock; 1.3.0, which
is what the entire fleet was running, does not. So the poll kept every 1.3.0
kiosk looking permanently busy and held back the very update that would have
fixed it. On 2026-09-05 a Romitan PC fetched `agent?v=1.3.1` (6 058 496 bytes)
at 20:20:10 and was still reporting 1.3.0 at 20:20:49 and after, because the
advert never let the room fall quiet. Only a reboot could have moved it.

So the advert is now read once, when the station home screen mounts. A school
changes this setting once or twice a year; a screen nobody has touched keeps
yesterday's advert until a student returns to it, the page reloads, or the PC
boots — each of which remounts the component and asks again. That is worth far
more than a heartbeat, because the idle window is the channel through which
every future agent fix reaches a classroom. `mobile-promo.test.tsx` pins it.

Rollback: retain prior API/web images. The added columns are compatible with
the previous API; do not run the down migration as part of application rollback.
