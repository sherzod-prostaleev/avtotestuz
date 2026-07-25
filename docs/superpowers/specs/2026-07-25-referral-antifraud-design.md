# Referral retroactive-attach antifraud — Design Note

Sana: 2026-07-25 · Milestone: M2 (M2-06 qoldig'i) · Qatlam: backend
Holat: **IMPLEMENTATSIYA QILINGAN** (2026-07-26). Bu hujjat yondashuvni qayd etadi.

> Kontekst: `2026-07-24-SESSION-HANDOFF.md` §3 bu bo'shliqni "past-xavfli, kelajakka" deb yozgan, **lekin** aynan shu
> yerda `ReferralCapture` qo'shilgani bu yo'lni **avvalgidan ochiqroq** qilganini ham qayd etgan. Bu hujjat shu
> qoldiqni yopish uchun.

---

## 1. Problem

Any logged-in profile can attach any referral code at any time, with no eligibility check beyond
"not your own code" and "you have not attached one before". `billing.Service.ApplyReferralCode`
(`backend/internal/billing/referral.go:80`) does exactly three things: resolve the code's owner, reject self-referral,
`INSERT INTO referral (... status 'pending')`. There is no check on the referee's account age, and — this is the part
usually assumed to be covered — **no check on the referee's payment history.**

The reward path makes that omission exploitable. `processReferralRewardOnPayment`
(`referral.go:143`) is called from `ProcessPaymentGrant` (`internal/billing/entitlement.go:195`) on **every**
completed payment, and it claims the referee's pending referral with
`ClaimPendingReferralForReferee` (`UPDATE referral SET status='rewarded' WHERE referee_id = $1 AND status='pending'`).
That statement is race-safe and fires at most once per referee — but "once per referee" is not "on the referee's
**first** payment". So:

> A user who signed up in January and has been renewing VIP for six months can attach a friend's code today, and their
> **next renewal** grants the friend 7 free VIP days. The friend acquired nobody.

Two existing customers can also trade: A attaches B's code, B attaches A's code, and each one's next renewal — a
purchase they were making anyway — hands the other 7 free VIP days. `referral.referee_id` is UNIQUE and
`ClaimPendingReferralForReferee` flips the status, so this is a one-time 7 days each, not a repeating cycle; but it is
7 days of revenue given away for zero customer acquisition, and it scales linearly with the size of a code-trading
group. The self-referral CHECK (`chk_no_self_referral`, migration `0015`) does not touch it, because the accounts are
genuinely different.

**Why the surface got wider, not narrower.** M2-10's audit fix (handoff §1.-1 #5) hardened code retention on purpose:
`lib/referral-storage.ts` now keeps a stored code through transient failures and only clears it on a **definitive**
server verdict (`referral_not_found` / `referral_self` / `referral_already_applied`), and the new `ReferralCapture`
component sits in the **authenticated** layout and retries the attach after login. That fix was correct — codes were
being lost in two places — but its consequence is that the attach path is now reachable and self-retrying from any
authenticated page, not only from the OTP-verify screen. Closing the eligibility hole therefore matters more than it
did when the handoff first filed it.

---

## 2. Options

| # | Option | Assessment |
|---|---|---|
| **A** | **Only brand-new users** — allow attach only if the profile has no prior activity at all (effectively "during the signup flow"). | Closes the hole completely, and breaks the flow that was just fixed: `referral-storage.ts` exists *because* codes legitimately survive OTP failures, network drops, and app reloads, and `ReferralCapture` retries minutes or hours later. "Only at signup" would make those retries fail and re-create the M2-10 bug from the other direction. |
| **B** | **Time window after signup** — allow attach while `profile.created_at > now() - N days`. | Preserves the retry flow, kills the "six-month customer attaches a code" case. On its own it leaks one case: a user who signs up, pays on day 1, then attaches a code on day 3 — they were never acquired by the referrer, but they are inside the window. |
| **C** | **First-payment-only.** | Widely assumed to already exist; it **does not**. What exists is "once per referee" (§1). Implementing it properly means either checking payment history at attach time, or checking it in the reward path. The reward path is money-critical code that was rewritten twice in audits (`ClaimPendingReferralForReferee`, `LockProfileForGrant`) — the handoff's money-code pattern applies, and touching it for a non-money guard is avoidable risk. |
| **D** | **Revive the unused `referral_attribution` table** (migration `0003`: `referee_id` PK, `referrer_id`, `reward_status` CHECK pending/granted/rejected, `fraud_flags jsonb`). | Rejected. M2-06 built a parallel `referral` table (migration `0015`) and everything — queries, service, handlers, tests, the frontend — reads that one. Reviving `referral_attribution` would create **two attribution truths** for the same fact, which is precisely the class of bug this project keeps finding (payment snapshots, leaderboard live-vs-rebuild). Its only unique asset is `fraud_flags jsonb`, and if fraud signals are ever needed they can be a column on `referral` for far less risk. Recommendation: a later migration **drops** `referral_attribution`, or the handoff records it as intentionally dead. |
| **E** | **Device / IP heuristics** — reject when referrer and referee share a `device.fingerprint` (migration `0002`) or an asserted client IP (`auth.NewClientIPResolver`). | Not as a hard block. In Uzbekistan, mobile CGNAT, shared family connections, and internet cafés make IP collision ordinary — the false-positive rate would punish real referrals (a student inviting a classmate on the same Wi-Fi is the *expected* use case, not the attack). Useful only as a **recorded signal** for M3 admin review, and M3 is last. |

---

## 3. Recommendation: B + C-at-attach-time, enforced in one place

**Rule:** `ApplyReferralCode` accepts a code only if **both** hold:

1. the referee has **no completed payment** (`payment.status = 'paid'`) — ever; and
2. `profile.created_at > now() - referral_attach_window_days`.

Rationale, in the order that decided it:

- **Condition 1 is the semantically correct rule for this incentive.** The referral program pays for *customer
  acquisition* (handoff §2: "referee birinchi to'lovidan keyin"). Someone who has already paid was not acquired by the
  referrer, whatever the calendar says. This is also the condition that actually closes the farming loop in §1 — a
  window alone does not.
- **Condition 2 keeps the reward tied to acquisition rather than to timing luck**, and bounds the blast radius: a
  never-paying account cannot sit dormant for two years and then be "sold" as a fresh referral to whoever is running
  a code-trading group.
- **Enforcing at attach, not at reward, is the low-risk placement.** The `referral` row never gets created, so the
  reward path — `ClaimPendingReferralForReferee` + `GrantDays`'s `LockProfileForGrant`, both products of audit fixes —
  is not touched at all. A guard that prevents bad data from existing is strictly safer than a guard that filters bad
  data later, and it also means the user finds out **immediately** instead of discovering months later that their
  friend never got a bonus.
- **No new schema.** `payment.status`, `profile.created_at`, and `limit_config` all exist. The window lives in
  `limit_config` as `referral_attach_window_days` (`free_value = vip_value = 30`) — same shape and same tunability as
  `grand_mock_min_studied_pct` (migration `0018`), so M3's admin panel can retune it without a deploy. `free_value ==
  vip_value` with the reason written in the migration comment, exactly as `0018` does, because a VIP dimension is
  meaningless here.
- **30 days, not 7.** The flow it must not break is `ReferralCapture` retrying a stored code after a failed attach,
  and a user who installs the PWA, gets distracted, and returns next week is normal behaviour. 30 days is generous
  enough that no legitimate invite is lost and short enough that dormant accounts are not tradeable.

### 3.1 What must change (not implemented — this is the shape)

1. **Migration** — `INSERT INTO limit_config (key, free_value, vip_value) VALUES ('referral_attach_window_days', 30, 30);`
   with a comment explaining why both values are equal.
2. **One new query** — `CountPaidPaymentsForProfile` (`SELECT count(*) FROM payment WHERE profile_id = $1 AND status = 'paid'`).
   `profile.created_at` comes from the existing profile read.
3. **`ApplyReferralCode`** — after the self-referral check and before `CreateReferral`, evaluate both conditions and
   return one of two new sentinel errors.
4. **Two new error codes**, because collapsing them into the existing ones would tell the user the wrong thing:
   - `referral_not_eligible_paid` → "sizda allaqachon to'lov tarixi bor"
   - `referral_window_closed` → "havolani ro'yxatdan o'tganingizdan keyin 30 kun ichida qo'llash mumkin"
5. **`frontend/src/lib/referral-storage.ts` — the step that must not be forgotten.** Both new codes must be added to
   the **definitive** verdict list, so the stored code is **cleared**. If they are not, `ReferralCapture` will retry
   forever on every authenticated page load: an infinite background loop of requests that can never succeed. This is
   the single highest-risk detail of the whole change, because it fails silently — nothing breaks visibly, the app
   just quietly hammers the endpoint.
6. **i18n** — three locales for both messages (`uz-Latn`, `uz-Cyrl`, `ru`), localized by `err.code` the way referral
   and promo errors already are.

### 3.2 Tests to lock it (TDD; each corresponds to a real exploit or a real regression)

- A profile with a `paid` payment cannot attach a code → `referral_not_eligible_paid`. *This is the §1 exploit; it
  must fail before the fix and pass after.*
- A profile created 31 days ago with no payments cannot attach → `referral_window_closed`.
- A profile created 3 days ago with no payments **can** attach → unchanged happy path.
- A profile with a `created` / `pending` / `failed` / `canceled` payment **can** still attach — an abandoned checkout
  is not a purchase, and blocking it would punish a user who merely opened the payment page.
- Existing `pending` referrals attached **before** this change still reward normally on the referee's next payment —
  the guard is at attach time only, and must not retroactively invalidate anything.
- Frontend: both new codes clear the stored code (`referral-storage.test.ts`), i.e. no retry loop.

### 3.3 Deliberately not included

- Device/IP blocking (option E) — record-only, and only when M3 admin exists to review it.
- Reviving or dropping `referral_attribution` — a separate cleanup; note it in the handoff either way so the next
  session does not rediscover an empty table with a promising name and build on it.
- Retroactive audit of already-granted referrals. Pre-launch user data was cleared (handoff §1), so there is nothing
  to reconcile; if this ships post-launch, a one-off report is a separate task.

---

## 4. Ochiq savol (Sherzod)

Shart 1 ("hech qachon to'lov qilmagan bo'lishi kerak") **qattiq** qoidami, yoki marketing sababli yumshatilsinmi
(masalan "eski mijoz ham do'st taklif qilishi mumkin, lekin mukofot faqat do'stning **o'z** birinchi to'lovidan
keladi")? Tavsiya — **qattiq qoldirish**: referal dasturi yangi mijoz jalb qilish uchun to'laydi, va yumshatilgan
variant §1'dagi ikki-akkaunt halqasini ochiq qoldiradi. Lekin bu biznes qarori, texnik qaror emas.
