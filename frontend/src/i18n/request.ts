import { getRequestConfig } from "next-intl/server";
import { notFound } from "next/navigation";
import { locales } from "./config";

export default getRequestConfig(async ({ requestLocale }) => {
  const requestedLocale = await requestLocale;
  if (!requestedLocale || !locales.includes(requestedLocale as (typeof locales)[number])) notFound();
  const locale = requestedLocale as (typeof locales)[number];
  return {
    locale,
    messages: (await import(`../../messages/${locale}.json`)).default,
  };
});
