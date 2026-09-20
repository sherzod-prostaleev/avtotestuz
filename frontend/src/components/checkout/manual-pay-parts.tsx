"use client";

import { useEffect, useMemo, useState } from "react";
import { useTranslations } from "next-intl";
import { AlertTriangle, Check, Copy, Timer, X } from "lucide-react";

export type ManualPayInfo = {
  payment_id: string;
  amount_uzs: number;
  pan_full: string;
  pan_last4: string;
  holder_name: string;
  network: string;
  hold_until: string;
  manual_state: string;
  payment_status?: string;
  /** Set once the payer has pressed "I paid". */
  claimed_at?: string | null;
};

export function formatSom(n: number): string {
  return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, " ");
}

export function formatPan(pan: string): string {
  const d = pan.replace(/\D/g, "");
  return d.replace(/(.{4})/g, "$1 ").trim();
}

/**
 * The wrong sum to show beside the right one — the one this payer is actually
 * about to send.
 *
 * Transfers are matched on (card last4, exact amount) alone, see
 * `FindManualPayMatchCandidate`, so the sum is the whole identity of a payment
 * and there are two ways to get it wrong.
 *
 * A plain tariff price (24 900) goes out rounded up: 25 000. It matches
 * nothing, and an admin has to untangle it by hand.
 *
 * A nudged price is the dangerous one. `uniqueManualAmount` adds 1–99 som only
 * when the plain price is *already open on that same card*, so 24 901 is proof
 * that someone else is currently owed exactly 24 900 there. That base is also
 * the number on the plan screen and in every advert, so it is the tempting
 * mistake — and sending it confirms the other person's payment instead of this
 * one. Whenever the sum carries a tail, that tail is what must be defended.
 */
export function wrongAmountDecoy(amountUzs: number): number {
  const tail = amountUzs % 100;
  if (tail !== 0) return amountUzs - tail;
  const up = Math.ceil(amountUzs / 1000) * 1000;
  return up === amountUzs ? amountUzs + 1000 : up;
}

/**
 * Read from the states the backend actually writes
 * (`manual_pay_assignment.manual_state`: awaiting_transfer → claimed →
 * awaiting_review → consumed | rejected). The phone body used to test for a
 * `"review"` that no query ever produces, so a learner who pressed "To'lov
 * qildim" was told nothing at all.
 */
export function isManualPaid(info: ManualPayInfo): boolean {
  return info.payment_status === "paid" || info.manual_state === "consumed";
}

/**
 * Whether this payer has already reported their transfer.
 *
 * `claimed_at` and not the state alone: an expired hold rewrites `claimed` to
 * `awaiting_review`, so a claim would otherwise look unmade. It matters in the
 * other direction too — `awaiting_review` on its own only means the hold ran
 * out, and someone who transferred late must still be able to say so, which
 * `ClaimManualPayAssignment` accepts.
 */
export function isManualClaimed(info: ManualPayInfo): boolean {
  return Boolean(info.claimed_at) || info.manual_state === "claimed";
}

export function useCountdown(untilIso: string): number {
  const until = useMemo(() => new Date(untilIso).getTime(), [untilIso]);
  const [left, setLeft] = useState(() => Math.max(0, Math.floor((until - Date.now()) / 1000)));
  useEffect(() => {
    const id = window.setInterval(() => {
      setLeft(Math.max(0, Math.floor((until - Date.now()) / 1000)));
    }, 1000);
    return () => window.clearInterval(id);
  }, [until]);
  return left;
}

/**
 * The sum, with the tail that makes it this payment underlined.
 *
 * A nudged sum differs from the advertised price by its last two digits and
 * nothing else — 24 901 against 24 900 — so those two digits are the whole
 * defence. Underlined rather than recoloured: the number must stay one number.
 */
function AmountWithTail({ value, markClassName = "" }: { value: number; markClassName?: string }) {
  const text = formatSom(value);
  if (value % 100 === 0) return <>{text}</>;
  return (
    <>
      {text.slice(0, -2)}
      <span className={`underline decoration-2 underline-offset-[3px] ${markClassName}`}>
        {text.slice(-2)}
      </span>
    </>
  );
}

/** Humo's palette, dark enough for white text in either theme. */
const CARD_GRADIENT = "linear-gradient(135deg, #0a3b2c 0%, #126a4c 48%, #1ea06f 100%)";

function ChipIcon() {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 32 24"
      className="h-[22px] w-[30px] shrink-0"
      fill="none"
    >
      <rect x="0.75" y="0.75" width="30.5" height="22.5" rx="4" fill="#E8C05A" stroke="#B8923C" strokeWidth="1.5" />
      <path
        d="M11 1v22M21 1v22M1 8h10M21 8h10M1 16h10M21 16h10"
        stroke="#B8923C"
        strokeWidth="1.3"
      />
    </svg>
  );
}

type CopyKind = "pan" | "amount";

function CardCopyButton({
  label,
  value,
  active,
  onCopied,
}: {
  label: string;
  value: string;
  active: boolean;
  onCopied: () => void;
}) {
  async function run() {
    try {
      await navigator.clipboard.writeText(value);
      onCopied();
    } catch {
      // A denied clipboard permission is not worth an error state — the number
      // is on screen and can be typed.
    }
  }

  return (
    <button
      type="button"
      aria-label={label}
      onClick={() => void run()}
      className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border border-white/25 bg-white/15 text-white transition-colors hover:bg-white/25 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/70"
    >
      {active ? (
        <Check aria-hidden="true" className="h-5 w-5" />
      ) : (
        <Copy aria-hidden="true" className="h-5 w-5" />
      )}
    </button>
  );
}

/**
 * The receiving card, drawn as a bank card.
 *
 * One body for the phone and the wide layout: the PAN and the sum a learner
 * must copy are the same two facts on both, and a second implementation of
 * money on screen is exactly the kind of thing that drifts.
 */
export function HumoPayCard({ info, className = "" }: { info: ManualPayInfo; className?: string }) {
  const t = useTranslations("ManualPay");
  const [copied, setCopied] = useState<CopyKind | null>(null);
  const left = useCountdown(info.hold_until);

  function markCopied(kind: CopyKind) {
    setCopied(kind);
    window.setTimeout(() => setCopied((prev) => (prev === kind ? null : prev)), 1500);
  }

  const network = (info.network || "humo").toUpperCase();
  const ended = left <= 0;
  const urgent = !ended && left <= 120;
  const clock = `${String(Math.floor(left / 60)).padStart(2, "0")}:${String(left % 60).padStart(2, "0")}`;

  return (
    <div
      className={`relative isolate shrink-0 overflow-hidden rounded-2xl px-4 py-2.5 text-white shadow-lg ${className}`}
      style={{ background: CARD_GRADIENT }}
    >
      {/* The embossed arcs a real card catches the light with. Purely paint. */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute -right-14 -top-20 h-44 w-44 rounded-full bg-white/[0.07]"
      />
      <div
        aria-hidden="true"
        className="pointer-events-none absolute -bottom-24 -left-12 h-44 w-44 rounded-full bg-black/10"
      />

      <div className="flex items-center justify-between gap-2">
        <span className="flex min-w-0 items-center gap-2">
          <ChipIcon />
          <span className="truncate font-display text-sm font-extrabold uppercase tracking-[0.25em] text-white/90">
            {network}
          </span>
        </span>
        {/* The window to pay, kept on the card itself so it is never the thing
            that scrolled out of sight. Deliberately not a live region: these
            digits change every second. */}
        <span
          className={`flex shrink-0 items-center gap-1.5 rounded-lg px-2 py-1 text-[13px] font-extrabold tabular-nums ${
            ended ? "bg-black/35 text-white/80" : urgent ? "bg-white/25 text-white" : "bg-black/25 text-white"
          }`}
        >
          <Timer aria-hidden="true" className="h-4 w-4 shrink-0" />
          {ended ? t("holdEnded") : clock}
        </span>
      </div>

      <div className="mt-2.5 flex items-center gap-2">
        {/* Shrinks with the screen rather than wrapping: sixteen digits split
            over two lines are sixteen digits someone can mistype. */}
        <span className="min-w-0 flex-1 font-mono text-[clamp(0.9rem,4.4vw,1.0625rem)] font-bold leading-none tracking-[0.06em] tabular-nums">
          {formatPan(info.pan_full)}
        </span>
        <CardCopyButton
          label={t("copyPanLabel")}
          value={info.pan_full}
          active={copied === "pan"}
          onCopied={() => markCopied("pan")}
        />
      </div>

      <p className="mt-1.5 truncate text-[11px] font-semibold uppercase leading-4 tracking-[0.1em] text-white/75">
        {info.holder_name}
      </p>

      <div className="mt-2.5 flex items-center gap-2 rounded-xl bg-black/25 px-3 py-1.5">
        <div className="min-w-0 flex-1">
          <p className="text-[11px] font-extrabold uppercase leading-none tracking-[0.1em] text-white/70">
            {t("exactAmount")}
          </p>
          <p className="mt-1 font-display text-[25px] font-extrabold leading-none tabular-nums">
            <AmountWithTail value={info.amount_uzs} markClassName="decoration-white/70" />
            <span className="ml-1 text-sm font-bold text-white/80">{t("somSuffix")}</span>
          </p>
        </div>
        <CardCopyButton
          label={t("copyAmountLabel")}
          value={String(info.amount_uzs)}
          active={copied === "amount"}
          onCopied={() => markCopied("amount")}
        />
      </div>

      <span aria-live="polite" className="sr-only">
        {copied ? t("copied") : ""}
      </span>
    </div>
  );
}

/**
 * "Send this sum, not the round one next to it."
 *
 * Prose already said as much and was read past — the reported failure is a
 * 24 900 checkout paid as 25 000. Two cells, one ticked and one crossed out,
 * say it without being read. Colour is never the only carrier: each cell has
 * its own icon and its own words.
 *
 * Neither sum may ever be cut short: a truncated "109 900" reads as a
 * perfectly plausible 109 90, and this screen exists to stop people sending
 * the wrong number. The digits are unbreakable; only the suffix may wrap.
 */
export function ExactAmountNotice({
  amountUzs,
  className = "",
}: {
  amountUzs: number;
  className?: string;
}) {
  const t = useTranslations("ManualPay");
  const som = t("somSuffix");
  const wrong = wrongAmountDecoy(amountUzs);

  return (
    <div className={`shrink-0 rounded-2xl border border-danger/45 bg-danger/[0.07] p-2.5 ${className}`}>
      <p className="flex items-center gap-2 font-display text-[15px] font-extrabold leading-tight text-danger-ink">
        <AlertTriangle aria-hidden="true" className="h-[18px] w-[18px] shrink-0" />
        {t("roundingTitle")}
      </p>

      {/* Side by side, because the point is the comparison: this one, not that
          one. Stacked, they read as two separate instructions. */}
      <dl className="mt-1.5 grid grid-cols-2 gap-2">
        <div className="min-w-0 rounded-xl border border-success/40 bg-success/10 px-2.5 py-1">
          <dt className="flex items-center gap-1.5">
            <Check aria-hidden="true" className="h-[18px] w-[18px] shrink-0 text-success-ink" />
            <span className="min-w-0 font-display text-base font-extrabold tabular-nums">
              <span className="whitespace-nowrap">
                <AmountWithTail value={amountUzs} markClassName="decoration-success-ink/80" />
              </span>{" "}
              {som}
            </span>
          </dt>
          <dd className="mt-0.5 text-xs font-semibold leading-tight text-success-ink">
            {t("roundingRight")}
          </dd>
        </div>

        <div className="min-w-0 rounded-xl border border-danger/40 bg-danger/10 px-2.5 py-1">
          <dt className="flex items-center gap-1.5">
            <X aria-hidden="true" className="h-[18px] w-[18px] shrink-0 text-danger-ink" />
            <span className="min-w-0 font-display text-base font-extrabold tabular-nums line-through decoration-danger-ink/70 decoration-2">
              <span className="whitespace-nowrap">{formatSom(wrong)}</span> {som}
            </span>
          </dt>
          <dd className="mt-0.5 text-xs font-semibold leading-tight text-danger-ink">
            {t("roundingWrong")}
          </dd>
        </div>
      </dl>
    </div>
  );
}

/**
 * What the clock on the card means.
 *
 * It used to read "Karta band: 09:23" — "card busy" — and people took it to
 * mean the card could not receive money, so they waited instead of paying. It
 * is the opposite: the window to pay into this card. One quiet line while it
 * runs, and the only instruction that matters once it has run out.
 */
export function HoldNote({ untilIso, className = "" }: { untilIso: string; className?: string }) {
  const t = useTranslations("ManualPay");
  const ended = useCountdown(untilIso) <= 0;

  if (ended) {
    return (
      <p
        role="status"
        className={`shrink-0 rounded-2xl border border-danger/40 bg-danger/[0.07] px-3 py-2 text-xs font-semibold leading-snug text-danger-ink ${className}`}
      >
        {t("holdEndedHint")}
      </p>
    );
  }

  return (
    <p className={`shrink-0 px-1 text-xs leading-snug text-muted-foreground ${className}`}>
      {t("payWindowHint")}
    </p>
  );
}
