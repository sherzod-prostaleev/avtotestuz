export const locales = ["uz-Latn", "uz-Cyrl", "ru"] as const;
export type Locale = (typeof locales)[number];
export const defaultLocale: Locale = "uz-Latn";
