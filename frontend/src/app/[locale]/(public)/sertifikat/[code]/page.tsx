"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { Award, ArrowLeft, Printer, Share2 } from "lucide-react";
import { apiGet, ApiError } from "@/lib/api-client";
import { BrandLogo } from "@/components/brand/brand-logo";
import { Button } from "@/components/ui/button";
import { certificateShareUrl, shareOrCopyCertificateLink } from "@/lib/certificate-share";

type PublicCert = {
  share_code: string;
  score: number;
  total: number;
  issued_at: string;
};

export default function PublicCertificatePage({
  params,
}: {
  params: { code: string };
}) {
  const t = useTranslations("GrandMock");
  const locale = useLocale();
  const code = params.code;
  const [cert, setCert] = useState<PublicCert | null>(null);
  const [missing, setMissing] = useState(false);
  const [loading, setLoading] = useState(true);
  const [shareHint, setShareHint] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      setMissing(false);
      try {
        const data = await apiGet<PublicCert>(`grand-mock/certificates/${encodeURIComponent(code)}`);
        if (!cancelled) setCert(data);
      } catch (err) {
        if (!cancelled) {
          setCert(null);
          setMissing(err instanceof ApiError && (err.status === 404 || err.code === "not_found"));
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [code]);

  const issuedLabel =
    cert?.issued_at &&
    t("certificatePublicIssued", {
      date: new Date(cert.issued_at).toLocaleDateString(locale, {
        year: "numeric",
        month: "long",
        day: "numeric",
      }),
    });

  async function onShare() {
    if (!cert) return;
    const url = certificateShareUrl(window.location.origin, locale, cert.share_code);
    try {
      const mode = await shareOrCopyCertificateLink({
        url,
        title: t("certificatePublicTitle"),
        text: t("certificatePublicBody", { score: cert.score, total: cert.total }),
      });
      setShareHint(mode === "clipboard" ? t("certificateCopied") : null);
    } catch {
      setShareHint(null);
    }
  }

  return (
    <main className="mx-auto flex min-h-screen w-full max-w-lg flex-col px-4 py-10 print:max-w-none print:px-0 print:py-0">
      <div className="mb-8 flex items-center justify-between print:hidden">
        <Link href={`/${locale}`} className="flex items-center gap-3">
          <BrandLogo size={36} className="h-9 w-9 rounded-2xl object-cover" />
          <span className="font-display text-lg font-bold">Driver Go</span>
        </Link>
        <Link
          href={`/${locale}`}
          className="inline-flex min-h-11 h-11 items-center justify-center rounded-xl border border-border bg-card px-4 text-xs font-bold tracking-wide text-foreground transition-all duration-150 hover:border-accent hover:text-foreground"
        >
          <ArrowLeft className="mr-1 h-4 w-4" aria-hidden />
          {t("certificateClose")}
        </Link>
      </div>

      <section className="rounded-3xl border border-gold/30 bg-card p-8 text-center shadow-sm print:rounded-none print:border print:border-black print:shadow-none">
        <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full border border-gold/40 bg-gold/15 print:border-black">
          <Award className="h-9 w-9 text-gold print:text-black" aria-hidden />
        </div>
        <h1 className="mt-5 font-display text-2xl font-extrabold">{t("certificatePublicTitle")}</h1>

        {loading ? (
          <p className="mt-4 text-sm text-muted-foreground">…</p>
        ) : missing || !cert ? (
          <p className="mt-4 text-sm text-muted-foreground">{t("certificatePublicMissing")}</p>
        ) : (
          <>
            <p className="mx-auto mt-3 max-w-sm text-sm text-muted-foreground print:text-black">
              {t("certificatePublicBody", { score: cert.score, total: cert.total })}
            </p>
            <p className="mt-4 font-mono text-xs text-muted-foreground print:text-black">{cert.share_code}</p>
            {issuedLabel ? (
              <p className="mt-2 text-xs text-muted-foreground print:text-black">{issuedLabel}</p>
            ) : null}
            <p className="mt-4 text-[11px] text-muted-foreground print:hidden">{t("certificatePrintHint")}</p>
            <div className="mt-4 flex flex-wrap justify-center gap-2 print:hidden">
              <Button type="button" variant="outline" size="sm" className="gap-2" onClick={() => void onShare()}>
                <Share2 className="h-4 w-4" aria-hidden />
                {t("certificateShareOrCopy")}
              </Button>
              <Button type="button" variant="outline" size="sm" className="gap-2" onClick={() => window.print()}>
                <Printer className="h-4 w-4" aria-hidden />
                {t("certificatePrint")}
              </Button>
            </div>
            {shareHint ? <p className="mt-2 text-xs text-muted-foreground print:hidden">{shareHint}</p> : null}
          </>
        )}
      </section>
    </main>
  );
}
