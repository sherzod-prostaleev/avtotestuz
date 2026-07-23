# AvtoTest Competitor Visual Audit

Date: 2026-07-22
Evidence set: 44 workspace-root screenshots, inspected individually at original resolution
Products visible: OsonPrava (18 mobile captures), Onless (21 desktop captures), legacy AvtoTest (5 desktop captures)

## Executive conclusion

The competitors are broad, but not uniformly polished. OsonPrava is strongest at mobile activation, large touch targets, voice commentary, social competition, and a single-screen learning menu. Onless is strongest at desktop information architecture, road-sign browsing, sequential ticket progression, pricing/payment clarity, and a genuinely rich post-answer explanation.

AvtoTest should not copy either visual system. It can outperform both with a calmer, accessible cross-device shell and a tighter learning loop:

1. **Resume or start in one action.** Put the active session and today's due review ahead of the full catalog.
2. **Explain, do not merely grade.** Use structured legal explanation blocks, option-by-option analysis, related signs, and an explicit source/reference layer after an answer.
3. **Make readiness actionable.** Turn readiness, due questions, weak categories, and streak into the next recommended action rather than decorative statistics.
4. **Use progressive disclosure.** Keep the default dashboard focused; move catalogs, social modes, payments, and settings behind clear secondary navigation.
5. **Win on trust and accessibility.** Honest data states, WCAG-aware contrast, keyboard support, visible focus, large touch targets, reduced motion, three complete locales, and server-side anti-cheat should be product features.

## Evidence limitations

- The files are static screenshots, so animation, latency, keyboard behavior, screen-reader semantics, error recovery, and full purchase completion cannot be verified.
- Five files show legacy AvtoTest on localhost rather than a competitor. They are retained because they are useful regression evidence.
- Some OsonPrava captures are long-page screenshots and some Onless captures are partial-scroll states. Findings only claim what is visible.
- The audit extracts interaction patterns and product strategy. It does not authorize copying competitor branding, illustrations, proprietary content, or trade dress.

## Flow audit

### 1. Public acquisition — OsonPrava

Evidence: `photo_1`, `photo_2`, `photo_3`.

Health: **Good activation, weak visual restraint**.

- Strong: registration-free promise, repeated primary CTA, Telegram and App Store paths, product-volume proof, free/ad-free claim, offline claim, and voice explanation are visible immediately.
- Weak: multiple competing CTAs, very saturated green and cyan, oversized copy, browser chrome inside the capture, repeated navigation, and low-contrast body text on dark navy.
- AvtoTest advantage: preserve the instant demo and proof, but use one dominant CTA, one secondary demo action, verified source/proof language, and responsive typography with fewer promotional claims.

### 2. Home and learning entry — OsonPrava and Onless

Evidence: `photo_18`, `Снимок экрана_20260717_222113`, `Снимок экрана_20260717_222142`.

Health: **Feature-rich but dense**.

- Strong: progress is visible, mistakes have a count badge, test start is obvious, and all major modes can be reached quickly.
- Weak: OsonPrava gives almost every feature equal weight; Onless uses many visually identical cards and two persistent announcement/VIP bars. Neither makes a personalized next-best action unmistakable.
- AvtoTest advantage: one "Continue" or "Review due questions" hero, followed by four core modes and a secondary library. Show why the recommendation matters and how long it will take.

### 3. Question solving and navigation

Evidence: `photo_14`, `photo_16`, `Снимок экрана_20260717_223252`.

Health: **Strong mechanics, inconsistent hierarchy**.

- Strong: timer, numbered navigator, F1-F5 shortcuts, bookmark/share/report controls, rule/learn access, large answers, image-first desktop layout, and a persistent question strip.
- Weak: the mobile header carries too many unlabeled icons; question text can dominate the entire viewport; desktop answers and image compete at equal weight; no visible autosave/resume status; timer is not paired with urgency semantics.
- AvtoTest advantage: sticky but compact session header, autosaved state, responsive split layout for image questions, text-width limits, explicit keyboard hints on desktop only, accessible labels, and clear unanswered/answered/current states that do not rely only on color.

### 4. Explanation and learning feedback

Evidence: `Снимок экрана_20260717_223315`, `Снимок экрана_20260717_223330`, `photo_2`, `photo_11`-`photo_13`.

Health: **Onless's strongest differentiator**.

- Strong: explanation is broken into meaning, important rule, visual description, where the rule applies, warning, answer analysis, final answer, and related signs. Helpful/not-helpful feedback provides a quality signal. OsonPrava monetizes audio commentary and key-word guidance.
- Weak: Onless's explanation is extremely long and visually noisy, with too many color boxes and emoji-like markers; the answer is easy to reveal accidentally if gating is weak. OsonPrava's voice feature is paywalled before its value is demonstrated.
- AvtoTest advantage: progressive explanation layers — a two-sentence result first, then "Why", "Rule/source", "Why other choices fail", and "Related signs". Provide optional audio later, but begin with safe, localized, server-gated text and explanation quality feedback.

### 5. Signs, tickets, saved, and mistakes

Evidence: `Снимок экрана_20260717_222401`, `222420`, `222431`, `222457`, `222519`, `222534`, `photo_15`.

Health: **Useful catalogs; empty states vary in quality**.

- Strong: sign code/name search, categorical filters, related-question counters, ticket mastery summary, sequential unlock explanation, and saved-empty guidance.
- Weak: sign cards are tiny on desktop; category dots rely on color; ticket grid has very low contrast for locked states; the mistakes empty state has no CTA; saved empty state explains the action but offers no direct practice route in OsonPrava.
- AvtoTest advantage: searchable real sign catalog with list/grid density controls, accessible text filters, a direct "Practice this sign" path, clear ticket prerequisites, and every empty state offering the most useful next action.

### 6. Monetization and payment

Evidence: `photo_10`-`photo_13`, `Снимок экрана_20260717_222905`, `222916`, `222932`.

Health: **Commercially mature, cognitively heavy**.

- Strong: plan duration, discount, daily equivalent, recommendation/popularity labels, benefit list, payment-method choice, promo entry, and support phone numbers reduce purchase anxiety.
- Weak: OsonPrava's car-based plan naming obscures product value; Onless shows five near-duplicate duration cards and substantial anchoring/discount noise. Neither screenshot clearly shows recurring-vs-one-time terms, refund policy, entitlement start, or a concise feature comparison.
- AvtoTest advantage: Free vs Premium comparison first, then 1/3/12-month duration; exact one-time/renewal terms; restore payment/history; local payment methods; honest savings math; and a post-payment success/resume path.

### 7. Social competition and services

Evidence: `photo_4`-`photo_8`, `Снимок экрана_20260717_222231`, `222309`, `222326`, `222338`, `222441`.

Health: **High engagement potential, high scope risk**.

- Strong: online-player presence, daily/weekly/monthly leaderboards, ranks, PvP matchmaking, invite/join, mentor marketplace, curator SLA, guarantee language, and Telegram linkage create retention and revenue surfaces.
- Weak: competition emphasizes raw point volume rather than learning quality; mixed-script usernames and unlabeled status badges are hard to scan; social features can distract from exam readiness; mentor/curator claims need operational proof.
- AvtoTest advantage: defer PvP and marketplace until the core loop is reliable. If introduced, rank by verified learning consistency/readiness improvement, add privacy controls, age-safe profiles, abuse reporting, and clear service SLAs.

### 8. Profile and settings

Evidence: `photo_9`, `photo_10`, `Снимок экрана_20260717_222157`, `222214`.

Health: **Broad coverage, uneven safety**.

- Strong: locale, theme, payment history, offline mode, profile editing, support, share/rate, security, subscription, receipts, and region fields are discoverable.
- Weak: destructive reset/delete actions sit close to routine settings; contrast is low; optional demographic fields add friction; OsonPrava mixes commercial plan upgrade into identity settings.
- AvtoTest advantage: separate Account, Appearance & language, Subscription & payments, Privacy & security, and Danger zone. Require confirmation/re-authentication for destructive operations and preserve locale across all flows.

### 9. Legacy AvtoTest regression evidence

Evidence: `111.png`, `Снимок экрана_20260720_235223`, `235241`, `Снимок экрана_20260721_000102`, `000143`.

Health: **Critical failure states**.

- Visible defects: raw Dart type error, raw "missing bearer token", giant low-information tiles, disabled-looking practice form with no useful state, huge unused space, debug ribbon, and no recovery beyond generic retry/home.
- Required bar: no raw backend/runtime strings in UI; authenticated-route guards; structured error codes mapped to localized recovery; useful skeleton/empty/error states; responsive content density; and production builds without debug markers.

## Individual image inventory

### OsonPrava mobile — 18/18

| File | Visible screen | Key evidence |
|---|---|---|
| `photo_1_2026-07-19_14-13-48.jpg` | Public CTA/footer | No-registration promise, app/Telegram entry, repeated CTA |
| `photo_2_2026-07-19_14-13-48.jpg` | Proof/features | 1220+ questions, 61 tickets, free, 3 languages, voice explanation |
| `photo_3_2026-07-19_14-13-48.jpg` | Public hero | Free/ad-free badge, smart learning, offline claim, two CTAs |
| `photo_4_2026-07-19_14-13-48.jpg` | Online players | Presence count, ranks, searching/in-game/score states |
| `photo_5_2026-07-19_14-13-48.jpg` | Monthly leaderboard | Podium, ranks, scores, monthly tab |
| `photo_6_2026-07-19_14-13-48.jpg` | Weekly leaderboard | Weekly tab and long ranked list |
| `photo_7_2026-07-19_14-13-48.jpg` | Daily leaderboard | Daily tab and podium/list hierarchy |
| `photo_8_2026-07-19_14-13-48.jpg` | Octagon/PvP hub | Matchmaking, online count, ranking, invites, match history |
| `photo_9_2026-07-19_14-13-48.jpg` | Profile/settings lower | Offline, reset, theme, support, Telegram, rate/share, delete/logout |
| `photo_10_2026-07-19_14-13-48.jpg` | Profile/settings upper | Avatar edit, user ID, free plan, upgrade, language, payments |
| `photo_11_2026-07-19_14-13-48.jpg` | One-week paywall | Bottom sheet, duration tabs, audio benefits, weekly price |
| `photo_12_2026-07-19_14-13-48.jpg` | Two-month paywall | Duration selection and recalculated price |
| `photo_13_2026-07-19_14-13-48.jpg` | One-month paywall | Popular badge, discount, benefit list |
| `photo_14_2026-07-19_14-13-48.jpg` | Long practice question | Timer, 1..N strip, F1-F5 answers, rule and learn actions |
| `photo_15_2026-07-19_14-13-48.jpg` | Saved empty | Instructional empty state without a direct CTA |
| `photo_16_2026-07-19_14-13-48.jpg` | Timed question | Bookmark/share/settings/report, timer, 3 answers |
| `photo_17_2026-07-19_14-13-48.jpg` | Random-count sheet | 50/100/200/500 question choices |
| `photo_18_2026-07-19_14-13-48.jpg` | Main learning dashboard | Progress, voice, mistakes badge, test, PvP, topics, tickets, exam, saved, signs |

### Onless desktop — 21/21

| File suffix | Visible screen | Key evidence |
|---|---|---|
| `222113.png` | Dashboard upper | Daily goal, onboarding prompt, quick-entry cards, announcements |
| `222142.png` | Dashboard lower | Signs, image questions, Telegram sync, Grand Mock lock, empty history |
| `222157.png` | Profile settings | Identity form, optional region/date/phone fields, settings subnav |
| `222214.png` | Locale/theme settings | Four locale choices and light/dark selection |
| `222231.png` | Academy course | Schedule, guarantee, certificate, mentor CTA |
| `222309.png` | Curator product | SLA, refund guarantee, weekly report, daily-equivalent pricing |
| `222326.png` | Mentor marketplace | Search/filter, mentor cards, price/language tags |
| `222338.png` | Telegram Quiz link | Two-step linking and six-digit verification |
| `222401.png` | Sign catalog, all | Search, categories, sign cards, related-question badges |
| `222420.png` | Sign catalog, mandatory | Filtered category and sparse-result state |
| `222431.png` | Saved empty | Explanation plus start-exam CTA |
| `222441.png` | Battle hub | Create or join battle |
| `222457.png` | Mistakes empty | Trophy empty state without CTA |
| `222519.png` | Ticket catalog | 63 tickets, mastery/completion, filters, locked sequence |
| `222534.png` | Ticket lock modal | Explicit 10/20 prerequisite and route to required ticket |
| `222905.png` | Pricing upper | Goal-based plans, discount anchors, daily price, support |
| `222916.png` | Pricing lower | Five duration tiers, recommendation/popularity, promo entry |
| `222932.png` | Payment/support | Payme/card choice, payment CTA, phone consultation |
| `223252.png` | Question workspace | Split answers/image, shortcuts, navigator, save, expert analysis |
| `223315.png` | Explanation upper | Rule decomposition, helpfulness feedback, structured blocks |
| `223330.png` | Explanation lower | warnings, option analysis, final answer, related signs |

### Legacy AvtoTest — 5/5

| File | Visible screen | Regression evidence |
|---|---|---|
| `111.png` | Ticket route crash | Raw null/int type error and debug ribbon |
| `Снимок экрана_20260720_235223.png` | Legacy home | Oversized feature tiles with minimal hierarchy |
| `Снимок экрана_20260720_235241.png` | Legacy practice setup | Sparse form, unclear disabled state, excessive empty space |
| `Снимок экрана_20260721_000102.png` | Authentication failure | Raw missing bearer token |
| `Снимок экрана_20260721_000143.png` | Admin/ticket crash | Raw null/int type error and debug ribbon |

## Competitive feature matrix

| Capability | OsonPrava | Onless | AvtoTest target |
|---|---:|---:|---|
| Registration-free first value | Strong | Not evidenced | Strong demo with no answer leak |
| Personalized next action | Partial | Partial | Readiness-driven recommendation + resume |
| Timed exam fidelity | Strong | Strong | Strong, server-authoritative timing and stop rules |
| Question ergonomics | Strong mobile | Strong desktop | Responsive split/single-column with autosave |
| Structured explanation | Limited in evidence | Very strong | Equally deep, calmer, sourced, progressively disclosed |
| Audio explanation | Strong/paid | Not evidenced | Later premium, accessible transcript always present |
| Spaced repetition | Claimed smart learning | Not evidenced | Real FSRS queue with transparent due state |
| Signs catalog | Entry only | Strong | Real searchable catalog + related practice |
| Saved/mistakes | Present | Present | Complete states, undo/retry, direct next action |
| Social/PvP | Very strong | Basic | Defer until learning core is excellent |
| Mentors/services | Not evidenced | Strong | Defer; only with verified operations/SLA |
| Pricing/payment | Mobile sheet | Mature web | Simpler comparison, exact terms, local payment flow |
| Accessibility | Weak visual evidence | Weak visual evidence | WCAG 2.2 AA target and keyboard/mobile parity |
| Localization | 3 claimed | 4 shown | Complete UZ Latin, UZ Cyrillic, RU with no hardcoded UI strings |
| Trust/security | Not visible | Not visible | Server gating, honest claims/data, localized safe errors |

## Prioritized product backlog

### P0 — required before paid beta

1. Reliable session start, persistence, resume, expiry, finish, and server-side answer-key gating.
2. Responsive question workspace with autosave status, accessible navigator, image zoom, keyboard shortcuts, report/save, and error recovery.
3. Structured post-answer explanation with legal/source references and no pre-answer semantic leak.
4. Dashboard next-best action: resume active session, review due FSRS items, or start recommended mode.
5. Complete saved, mistakes, ticket, practice, exam, history, and profile flows with real DTOs and localized loading/error/empty states.
6. Remove all raw errors, debug markers, fake fallbacks, unsupported claims, hardcoded interface strings, and inaccessible icon-only controls.
7. Honest signs state: ship a verified catalog or hide the catalog promise until data exists.

### P1 — launch differentiation

1. Explanation layers: quick reason, rule/source, option analysis, related signs, quality feedback.
2. Readiness plan with daily time estimate and weak-category actions.
3. Searchable signs catalog with filters and related-question counts once verified data is available.
4. Transparent Free/Premium comparison, local payment integration, payment history, restore/retry/success flows.
5. Offline-friendly cached session shell and resilient reconnect behavior where platform support permits.

### P2 — growth after core reliability

1. Audio explanations with synchronized transcript and playback controls.
2. Privacy-safe leaderboards and learning-focused challenges.
3. Friend battles only after anti-abuse, matchmaking, reconnect, and fairness rules exist.
4. Mentor/curator marketplace only after supply, moderation, service SLA, refunds, and support operations are real.

## Definition of "better than all screenshots"

The target is not more cards or more neon. AvtoTest is better when a first-time learner can start in under 10 seconds, an existing learner can resume in one action, every answer produces a trustworthy explanation, every failure has a safe recovery, every screen works in all three locales and at mobile/desktop sizes, answer keys cannot leak, and the system clearly tells the learner what to do next and why.
