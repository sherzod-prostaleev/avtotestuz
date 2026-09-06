import type { Metadata, Viewport } from "next";
import { Baloo_2, Manrope } from "next/font/google";
import { ViewTransitions } from "@/components/layout/view-transitions";
import { NextIntlClientProvider } from "next-intl";
import { getMessages, getTranslations } from "next-intl/server";
import { notFound } from "next/navigation";
import { Providers } from "@/app/providers";
import { locales, type Locale } from "@/i18n/config";
import { COMMON_NAMESPACES } from "@/i18n/namespaces";
import { pickMessages } from "@/i18n/pick-messages";
import { SITE_URL, SITE_NAME, canonicalUrl, buildLanguageAlternates } from "@/lib/seo";
import "../globals.css";

/** Notch / home-indicator safe areas (landing sticky CTA + app chrome). */
export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  viewportFit: "cover",
  themeColor: [
    { media: "(prefers-color-scheme: dark)", color: "#0E1218" },
    { media: "(prefers-color-scheme: light)", color: "#F1F3F6" },
  ],
};

const baloo = Baloo_2({
  subsets: ["latin", "latin-ext"],
  weight: ["600", "700", "800"],
  variable: "--font-baloo",
  display: "swap",
});
const manrope = Manrope({
  subsets: ["latin", "latin-ext", "cyrillic"],
  weight: ["400", "500", "600", "700"],
  variable: "--font-manrope",
  display: "swap",
});

export async function generateMetadata({ params }: {
  params: Promise<{ locale: string }>;
}): Promise<Metadata> {
  const { locale } = await params;
  if (!locales.includes(locale as Locale)) return {};
  const loc = locale as Locale;
  const t = await getTranslations({ locale, namespace: "Metadata" });
  const title = t("title");
  const description = t("description");
  const url = canonicalUrl(loc);
  return {
    metadataBase: new URL(SITE_URL),
    title: { default: title, template: `%s — ${SITE_NAME}` },
    description,
    alternates: {
      canonical: url,
      languages: buildLanguageAlternates(),
    },
    openGraph: {
      type: "website",
      siteName: SITE_NAME,
      title,
      description,
      url,
      locale: loc.replace("-", "_"),
      images: [{ url: "/logo-512.png", width: 512, height: 512, alt: SITE_NAME }],
    },
    twitter: {
      card: "summary",
      title,
      description,
      images: ["/logo-512.png"],
    },
    manifest: "/manifest.webmanifest",
    appleWebApp: {
      capable: true,
      title: "Driver Go",
      statusBarStyle: "black-translucent",
    },
    icons: {
      icon: [
        { url: "/favicon.ico", sizes: "48x48" },
        { url: "/favicon-32.png", type: "image/png", sizes: "32x32" },
        { url: "/favicon-16.png", type: "image/png", sizes: "16x16" },
      ],
      shortcut: "/favicon.ico",
      apple: "/apple-touch-icon.png",
    },
  };
}

const ORGANIZATION_JSON_LD = {
  "@context": "https://schema.org",
  "@type": "Organization",
  name: SITE_NAME,
  url: SITE_URL,
  logo: `${SITE_URL}/logo-512.png`,
};

const WEBSITE_JSON_LD = {
  "@context": "https://schema.org",
  "@type": "WebSite",
  name: SITE_NAME,
  url: SITE_URL,
};

export default async function LocaleLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ locale: string }>;
}) {
  const { locale } = await params;
  if (!locales.includes(locale as Locale)) notFound();
  const messages = await getMessages();

  return (
    <html lang={locale} suppressHydrationWarning className={`${baloo.variable} ${manrope.variable}`}>
      <head>
        {/* Default CSS is dark; script keeps next-themes class in sync before paint. */}
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var d=document.documentElement;var t=localStorage.getItem("theme");if(t==="light"){d.classList.add("light");d.classList.remove("dark");}else{d.classList.add("dark");d.classList.remove("light");}}catch(e){document.documentElement.classList.add("dark");}})();`,
          }}
        />
        <link rel="icon" href="/favicon.ico" sizes="48x48" />
        <link rel="icon" type="image/png" sizes="32x32" href="/favicon-32.png" />
        <link rel="icon" type="image/png" sizes="16x16" href="/favicon-16.png" />
        <link rel="apple-touch-icon" href="/apple-touch-icon.png" />
        <script
          type="application/ld+json"
          dangerouslySetInnerHTML={{ __html: JSON.stringify(ORGANIZATION_JSON_LD) }}
        />
        <script
          type="application/ld+json"
          dangerouslySetInnerHTML={{ __html: JSON.stringify(WEBSITE_JSON_LD) }}
        />
      </head>
      <body className="min-h-screen overflow-x-clip bg-background text-foreground antialiased" suppressHydrationWarning>
        <NextIntlClientProvider messages={pickMessages(messages, COMMON_NAMESPACES)}>
          <Providers>
            <ViewTransitions>{children}</ViewTransitions>
          </Providers>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
