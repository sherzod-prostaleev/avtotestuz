"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { Award, ArrowLeft } from "lucide-react";
import { apiGet, ApiError } from "@/lib/api-client";
import { BrandLogo } from "@/components/brand/brand-logo";
import { Button } from "@/components/ui/button";

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

  return (
    <main className="mx-auto flex min-h-screen w-full max-w-lg flex-col px-4 py-10">
      <div className="mb-8 flex items-center justify-between">
        <Link href={`/${locale}`} className="flex items-center gap-3">
          <BrandLogo size={36} className="h-9 w-9 rounded-2xl object-cover" />
          <span className="font-display text-lg font-bold">Driver Go</span>
        </Link>
        <Button asChild variant="outline" size="sm">
          <Link href={`/${locale}`}>
            <ArrowLeft className="mr-1 h-4 w-4" aria-hidden />
            {t("certificateClose")}
          </Link>
        </Button>
      </div>

      <section className="rounded-3xl border border-gold/30 bg-card p-8 text-center shadow-sm">
        <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-full border border-gold/40 bg-gold/15">
          <Award className="h-9 w-9 text-gold" aria-hidden />
        </div>
        <h1 className="mt-5 font-display text-2xl font-extrabold">{t("certificatePublicTitle")}</h1>

        {loading ? (
          <p className="mt-4 text-sm text-muted-foreground">…</p>
        ) : missing || !cert ? (
          <p className="mt-4 text-sm text-muted-foreground">{t("certificatePublicMissing")}</p>
        ) : (
          <>
            <p className="mx-auto mt-3 max-w-sm text-sm text-muted-foreground">
              {t("certificatePublicBody", { score: cert.score, total: cert.total })}
            </p>
            <p className="mt-4 font-mono text-xs text-muted-foreground">{cert.share_code}</p>
            {issuedLabel ? <p className="mt-2 text-xs text-muted-foreground">{issuedLabel}</p> : null}
          </>
        )}
      </section>
    </main>
  );
}
