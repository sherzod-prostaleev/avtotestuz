import Link from "next/link";
import { Home } from "lucide-react";
import { getLocale, getTranslations } from "next-intl/server";

export default async function NotFound() {
  const [locale, t] = await Promise.all([getLocale(), getTranslations("Errors")]);

  return (
    <main className="grid min-h-[70vh] place-items-center px-6 py-12">
      <section className="w-full max-w-lg rounded-3xl border bg-card p-8 text-center shadow-sm">
        <p className="font-display text-6xl font-extrabold text-primary" aria-hidden="true">
          404
        </p>
        <h1 className="mt-4 font-display text-3xl font-extrabold">{t("notFoundTitle")}</h1>
        <p className="mt-3 text-muted-foreground">{t("notFoundMessage")}</p>
        <Link
          href={`/${locale}`}
          className="mt-6 inline-flex min-h-11 items-center justify-center rounded-xl border-b-4 border-accent-shadow bg-accent px-5 text-sm font-bold tracking-wide text-accent-foreground shadow-3d transition-all duration-150 hover:brightness-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring active:translate-y-1 active:shadow-none"
        >
          <Home className="mr-2 h-4 w-4" aria-hidden="true" />
          {t("backHome")}
        </Link>
      </section>
    </main>
  );
}
