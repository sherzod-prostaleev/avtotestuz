import { NextIntlClientProvider } from "next-intl";
import { getLocale, getMessages } from "next-intl/server";
import { pickMessages } from "@/i18n/pick-messages";

export async function ClientMessages({
  namespaces,
  children,
}: {
  namespaces: readonly string[];
  children: React.ReactNode;
}) {
  const [locale, messages] = await Promise.all([getLocale(), getMessages()]);
  return (
    <NextIntlClientProvider locale={locale} messages={pickMessages(messages, namespaces)}>
      {children}
    </NextIntlClientProvider>
  );
}
