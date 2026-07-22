import type { Metadata } from "next";
import { Baloo_2, Manrope } from "next/font/google";
import { NextIntlClientProvider } from "next-intl";
import { getMessages } from "next-intl/server";
import { notFound } from "next/navigation";
import { Providers } from "@/app/providers";
import { locales, type Locale } from "@/i18n/config";
import "../globals.css";

const baloo = Baloo_2({ subsets: ["latin"], weight: ["600", "700", "800"], variable: "--font-baloo" });
const manrope = Manrope({ subsets: ["latin"], weight: ["400", "500", "600", "700"], variable: "--font-manrope" });

export const metadata: Metadata = {
  title: "AvtoTest — Haydovchilik nazariy imtihoniga tayyorgarlik",
  description: "O'zbekiston Respublikasining 1235 ta rasmiy YHQ savollari, 61 bilet va FSRS aqlli xatolar banki",
};

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
