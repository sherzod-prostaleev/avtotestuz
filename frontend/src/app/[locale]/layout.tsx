import type { Metadata } from "next";
import { Baloo_2, Manrope } from "next/font/google";
import { NextIntlClientProvider } from "next-intl";
import { getMessages, getTranslations } from "next-intl/server";
import { notFound } from "next/navigation";
import { Providers } from "@/app/providers";
import { locales, type Locale } from "@/i18n/config";
import "../globals.css";

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

export async function generateMetadata({
  params: { locale },
}: {
  params: { locale: string };
}): Promise<Metadata> {
  if (!locales.includes(locale as Locale)) return {};
  const t = await getTranslations({ locale, namespace: "Metadata" });
  return {
    title: t("title"),
    description: t("description"),
    manifest: "/manifest.webmanifest",
    themeColor: [
      { media: "(prefers-color-scheme: dark)", color: "#0E1218" },
      { media: "(prefers-color-scheme: light)", color: "#F1F3F6" },
    ],
    appleWebApp: {
      capable: true,
      title: "Driver Go",
      statusBarStyle: "black-translucent",
    },
    icons: {
      icon: "/logo.svg",
      shortcut: "/logo.svg",
      apple: "/apple-touch-icon.png",
    },
  };
}

export default async function LocaleLayout({
  children,
  params: { locale },
}: {
  children: React.ReactNode;
  params: { locale: string };
}) {
  if (!locales.includes(locale as Locale)) notFound();
  const messages = await getMessages();

  return (
    <html lang={locale} suppressHydrationWarning className={`${baloo.variable} ${manrope.variable}`}>
      <head>
        {/* Paint dark class before hydration — defaultTheme is dark; without this,
            locale navigations remount <html> and flash :root (light) background. */}
        <script
          dangerouslySetInnerHTML={{
            __html: `(function(){try{var t=localStorage.getItem("theme");if(t!=="light"){document.documentElement.classList.add("dark");document.documentElement.style.colorScheme="dark";}else{document.documentElement.classList.remove("dark");document.documentElement.style.colorScheme="light";}}catch(e){document.documentElement.classList.add("dark");document.documentElement.style.colorScheme="dark";}})();`,
          }}
        />
        <link rel="icon" type="image/svg+xml" href="/logo.svg" />
        <link rel="shortcut icon" href="/logo.svg" />
        <link rel="apple-touch-icon" href="/apple-touch-icon.png" />
      </head>
      <body className="min-h-screen bg-background text-foreground antialiased">
        <NextIntlClientProvider messages={messages}>
          <Providers>
            {children}
          </Providers>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
