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

  useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout>;
    let controller: AbortController | undefined;
    async function refresh() {
      if (document.visibilityState === "hidden") {
        timer = setTimeout(refresh, 60000);
        return;
      }
      controller = new AbortController();
      const timeout = setTimeout(() => controller?.abort(), 10000);
      try {
        const res = await fetch("/api/proxy/me/mobile-promo", {
          cache: "no-store",
          signal: controller.signal,
        });
        if (!res.ok) throw new Error("promotion_unavailable");
        const body = await res.json();
        if (!cancelled) setPromo(body.data);
      } catch {
        // Advertising must never block a lesson or retain a stale referral
        // after the school disabled it while this station was disconnected.
        if (!cancelled) setPromo(null);
      } finally {
        clearTimeout(timeout);
        if (!cancelled) timer = setTimeout(refresh, 60000 + Math.random() * 15000);
      }
    }
    void refresh();
    return () => {
      cancelled = true;
      clearTimeout(timer);
      controller?.abort();
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
