"use client";

import { useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";
import { BookOpen, Camera, ScanLine, Smartphone, Sparkles } from "lucide-react";

export type MobilePromo = { enabled: boolean; url: string; qr_data_url?: string };

// A promotion is only ever drawn from a PNG the backend generated itself from
// the exact saved link. Anything else -- a school that saved a URL but has no
// QR yet, a body from a proxy that answered with something other than the API
// -- renders nothing rather than an empty frame on a classroom wall.
export function isShowablePromo(promo: MobilePromo | null | undefined): promo is MobilePromo {
  return Boolean(
    promo?.enabled && promo.url && promo.qr_data_url?.startsWith("data:image/png;base64,"),
  );
}

export function MobilePromoBanner({ promo }: { promo: MobilePromo }) {
  const t = useTranslations("MobilePromo");
  if (!isShowablePromo(promo)) return null;
  return (
    <aside
      aria-label={t("title")}
      className="relative isolate overflow-hidden rounded-3xl border border-accent/25 bg-card p-5 shadow-lg sm:p-7"
    >
      <div
        aria-hidden="true"
        className="pointer-events-none absolute -left-20 -top-32 -z-10 h-80 w-80 rounded-full bg-accent/10 blur-3xl"
      />
      <div className="flex flex-col items-center gap-6 sm:flex-row sm:justify-between">
        <div className="min-w-0 flex-1 space-y-3 text-center sm:text-left">
          <div className="flex items-center justify-center gap-3 sm:justify-start" aria-hidden="true">
            <span className="flex h-12 w-12 -rotate-6 items-center justify-center rounded-2xl border-2 border-white bg-accent text-accent-foreground shadow-md">
              <Smartphone className="h-7 w-7" />
            </span>
            <span className="flex h-10 w-10 rotate-6 items-center justify-center rounded-xl bg-amber-100 text-amber-800 shadow-sm">
              <BookOpen className="h-5 w-5" />
            </span>
            <Sparkles className="h-6 w-6 text-accent" />
          </div>
          <p className="text-xs font-bold uppercase tracking-widest text-accent">{t("eyebrow")}</p>
          <h2 className="max-w-xl font-display text-2xl font-bold leading-tight tracking-tight sm:text-3xl">
            {t("title")}
          </h2>
          <p className="max-w-lg text-sm leading-relaxed text-muted-foreground">
            {t("description")}
          </p>
          <p className="flex items-center justify-center gap-2 text-sm font-semibold sm:justify-start">
            <Camera aria-hidden="true" className="h-4 w-4 shrink-0 text-accent" />
            {t("scan")}
          </p>
        </div>
        <figure className="flex shrink-0 flex-col items-center gap-2">
          <div className="rounded-2xl border border-black/10 bg-white p-2 shadow-sm">
            {/* The server creates the PNG locally from the exact saved URL. */}
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={promo.qr_data_url}
              width={192}
              height={192}
              alt={t("qrAlt")}
              className="h-48 w-48 [image-rendering:pixelated]"
            />
          </div>
          <figcaption className="flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
            <ScanLine aria-hidden="true" className="h-4 w-4" />
            {t("caption")}
          </figcaption>
        </figure>
      </div>
    </aside>
  );
}

/**
 * The always-visible half of the advert: a strip pinned to the bottom edge of
 * the kiosk screen.
 *
 * The rich banner alone did not work. The station home screen is 1197px tall
 * and every classroom resolution we ship to is shorter than that (1366x768,
 * 1280x800, 1024x768), so the banner began at y=816 -- entirely below the fold
 * on all three. A student who never scrolls, which is every student who came
 * to sit an exam, would never see the QR at all, and an advert nobody sees is
 * not an advert. This strip is what makes "permanently at the bottom, like an
 * ad" true; the banner underneath is what makes it worth looking at.
 *
 * It is aria-hidden on purpose: it is a second copy of a QR that is already in
 * the page, and announcing the same link twice helps nobody.
 *
 * Sized for one audience only. Unlike the banner -- which the admin preview
 * also renders, on operators' phones -- this strip exists solely inside the
 * station agent's Chromium --app window on a classroom PC, so it carries no
 * phone breakpoints; min-w-0/truncate are what keep it tidy if a teacher
 * restores that window to something narrow.
 *
 * No backdrop-filter and no will-change here. This element is painted on every
 * frame of every scroll on a low-end classroom PC, and compositor hints on
 * always-mounted chrome are exactly what made the site crawl in 2026-08-31.
 */
function MobilePromoStrip({ promo, muted }: { promo: MobilePromo; muted: boolean }) {
  const t = useTranslations("MobilePromo");
  return (
    <div
      aria-hidden="true"
      data-testid="mobile-promo-strip"
      // z-30 keeps it under the kiosk's own language/theme controls (z-40).
      className={`fixed inset-x-0 bottom-0 z-30 border-t border-accent/25 bg-card shadow-[0_-6px_24px_-12px_rgba(0,0,0,0.45)] transition-opacity duration-300 ${
        muted ? "pointer-events-none opacity-0" : "opacity-100"
      }`}
    >
      <div className="mx-auto flex max-w-6xl items-center gap-5 px-6 py-3">
        <div className="shrink-0 rounded-2xl border border-black/10 bg-white p-1.5 shadow-sm">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src={promo.qr_data_url}
            width={96}
            height={96}
            alt=""
            className="h-24 w-24 [image-rendering:pixelated]"
          />
        </div>
        <div className="min-w-0 flex-1 space-y-1">
          <p className="text-xs font-bold uppercase tracking-widest text-accent">{t("eyebrow")}</p>
          <p className="line-clamp-2 font-display text-lg font-bold leading-snug tracking-tight">
            {t("title")}
          </p>
          <p className="flex items-center gap-1.5 text-sm font-semibold text-muted-foreground">
            <Camera aria-hidden="true" className="h-4 w-4 shrink-0 text-accent" />
            <span className="truncate">{t("scan")}</span>
          </p>
        </div>
        <Smartphone
          aria-hidden="true"
          className="hidden h-10 w-10 shrink-0 -rotate-6 rounded-2xl border-2 border-white bg-accent p-2 text-accent-foreground shadow-md md:block"
        />
      </div>
    </div>
  );
}

export function StationMobilePromo() {
  const [promo, setPromo] = useState<MobilePromo | null>(null);
  const bannerRef = useRef<HTMLDivElement | null>(null);
  const [bannerOnScreen, setBannerOnScreen] = useState(false);

  // Asked once, when this screen mounts. There is deliberately no polling loop.
  //
  // The first version of this polled every ~65 seconds so that flipping the
  // switch in the admin console lit up an idle classroom on its own. That cost
  // far more than it bought. The agent's updater only installs a staged build
  // after the kiosk has made no proxied API call for 30 minutes -- that is how
  // it knows the room is empty and nobody is mid-exam -- and a request every
  // 65 seconds means that half-hour of quiet never arrives. Agent 1.3.1
  // excludes this path from its idle clock, but 1.3.0 does not, so on the
  // whole existing fleet the poll was holding back the very update that would
  // have fixed it: on 2026-09-05 a Romitan PC downloaded 1.3.1 at 20:20:10 and
  // was still running 1.3.0 afterwards, because the advert kept the room
  // looking busy. One heartbeat is never worth blocking the channel that
  // delivers fixes to a classroom.
  //
  // The trade is small: a school changes this once or twice a year, and a
  // screen nobody has touched keeps yesterday's advert until a student comes
  // back to it, the page reloads, or the PC boots. Every one of those remounts
  // this component and asks again.
  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 10000);
    (async () => {
      try {
        const res = await fetch("/api/proxy/me/mobile-promo", {
          cache: "no-store",
          signal: controller.signal,
        });
        if (!res.ok) throw new Error("promotion_unavailable");
        const body = await res.json();
        if (!cancelled) setPromo(body.data);
      } catch {
        // Advertising must never block a lesson: an unreachable or unhappy
        // backend leaves the station screen exactly as it is without it.
        if (!cancelled) setPromo(null);
      } finally {
        clearTimeout(timeout);
      }
    })();
    return () => {
      cancelled = true;
      clearTimeout(timeout);
      controller.abort();
    };
  }, []);

  // Once the full banner is on screen the pinned strip is showing the same QR
  // twice, so it fades out and gives the banner the bottom of the screen back.
  useEffect(() => {
    const node = bannerRef.current;
    // Guarded rather than assumed: jsdom has no IntersectionObserver, and a
    // missing one must leave the strip visible, not crash the kiosk home page.
    if (!node || typeof IntersectionObserver === "undefined") return;
    const observer = new IntersectionObserver(
      ([entry]) => setBannerOnScreen(entry.isIntersecting),
      { threshold: 0.5 },
    );
    observer.observe(node);
    return () => observer.disconnect();
  }, [promo]);

  // A school with the advert switched off gets the station screen exactly as
  // it was before this feature existed: no strip, no reserved space, nothing.
  if (!isShowablePromo(promo)) return null;

  return (
    <>
      <div ref={bannerRef}>
        <MobilePromoBanner promo={promo} />
      </div>
      {/* Room for the pinned strip (~135px), so it can never cover the banner
          no matter where the student stops scrolling. */}
      <div aria-hidden="true" className="h-36" />
      <MobilePromoStrip promo={promo} muted={bannerOnScreen} />
    </>
  );
}
