# Kiosk mobile promotion (agent 1.3.1)

| Requirement | Implementation | Verification |
|---|---|---|
| REQ-001: attractive QR banner below the kiosk home content | MobilePromoBanner; three locales; no overlay over exams | Playwright at the three classroom resolutions (1366x768, 1280x800, 1024x768); first-screen screenshots |
| REQ-001b: the QR is on screen permanently, like an advert | Pinned MobilePromoStrip, fading out while the full banner is on screen; reserved space so it never covers the page end | Measured: the banner alone began at y=816 on 1366x768, 1280x800 and 1024x768 — below the fold on every classroom resolution. Playwright now asserts a >=64px QR fully inside the first screen at scrollY 0 |
| REQ-002: admin controls each school's URL and enabled flag | B2B school MobilePromoPanel; users.read / users.entitlements.grant; transactional audit | Admin HTTP integration and form tests |
| REQ-003: QR encodes the exact submitted URL | Local QR generation from original bytes; no redirects or normalization | Independent zbarimg decode, including query escaping and fragment |
| REQ-004: every kiosk sees only its own school's active promotion | Profile-bound station lookup; active school, station and licence required; disabled by default | Cross-school, disabled and suspended school integration tests |
| REQ-005: remote settings propagate to open home screens | Non-overlapping 60–75 second polls, 10 second timeout; stale banner removed on error | Polling, disable, failure and unmount tests |
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
on every school's kiosks at the next page load. Agent 1.3.1 exists for the side
effect, not the feature — a 1.3.0 agent serving the new page sees a proxied API
call every ~65 seconds and therefore never observes a 30-minute idle window, so
its staged update can only activate at the next boot. That is the normal path
for a classroom PC that is switched off daily, but a school that leaves PCs on
around the clock will sit on 1.3.0 until someone reboots them. Pre-1.1.0 /
manual-update / offline PCs cannot be claimed updated merely because a release
was deployed; inspect reported agent_version per school.

Rollback: retain prior API/web images. The added columns are compatible with
the previous API; do not run the down migration as part of application rollback.
