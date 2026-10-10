// English is the product default. Retain the source locales for comparison.
export const LOCALES = ["en", "ko", "zh"] as const;
export type Locale = (typeof LOCALES)[number];
export const DEFAULT_LOCALE: Locale = "en";

// Build-time selection also works with Next.js static export. Do not introduce
// cookies or request headers here: that would make the pages dynamic.
export function resolveLocale(raw: string | undefined = process.env.NEXT_PUBLIC_LOCALE): Locale {
  return LOCALES.includes(raw as Locale) ? (raw as Locale) : DEFAULT_LOCALE;
}

export function formattingLocale(locale: Locale): string {
  return { en: "en-GB", ko: "ko-KR", zh: "zh-CN" }[locale];
}

// Only presentation changes; timestamps and the user's timezone stay intact.
export const FORMAT_LOCALE = formattingLocale(resolveLocale());
