import { DEFAULT_LOCALE, formattingLocale, resolveLocale } from "./config.ts";
import assert from "node:assert/strict";
import test from "node:test";

test("English is the default and invalid locale values fall back safely", () => {
  assert.equal(DEFAULT_LOCALE, "en");
  for (const raw of ["", "fr", "EN", "../../ko", " en "]) assert.equal(resolveLocale(raw), "en");
  for (const locale of ["en", "ko", "zh"]) assert.equal(resolveLocale(locale), locale);
});

test("formatting follows the selected locale without changing timestamps", () => {
  assert.equal(formattingLocale("en"), "en-GB");
  assert.equal(formattingLocale("ko"), "ko-KR");
  assert.equal(formattingLocale("zh"), "zh-CN");
  const date = new Date("2026-10-09T13:05:00Z");
  const original = date.getTime();
  const text = date.toLocaleString(formattingLocale("en"), {
    timeZone: "UTC",
    day: "numeric",
    month: "long",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
  assert.match(text, /9 October 2026/);
  assert.match(text, /13:05/);
  assert.equal(date.getTime(), original);
});
