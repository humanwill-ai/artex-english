import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { test } from "node:test";

const catalog = (locale) => JSON.parse(readFileSync(new URL(`../../messages/${locale}.json`, import.meta.url), "utf8"));
const warrantyHashes = {
  en: "57b7bf3f3d18bff835ce59f48e1b92796eba6c0de12450b4465df08929406ac9",
  ko: "a16a50d779c6fafc325fe6e53f46b4c479406f95f7f24633903524bb3efdb8d3",
  zh: "31e12a5309b02a4c45c4df0e04917e0ae06b229e8f89013696a21b4af828319d",
};

test("active login guidance stays aligned across selectable locales", () => {
  const terms = Object.fromEntries(["en", "ko", "zh"].map((locale) => [locale, catalog(locale).auth]));
  const keys = Object.keys(terms.en.terms).sort();
  for (const [locale, auth] of Object.entries(terms)) {
    assert.deepEqual(Object.keys(auth.terms).sort(), keys, `${locale} notice keys`);
    assert.match(auth.login.agree, /^.*<terms>.*<\/terms>.*$/, `${locale} acknowledgement link`);
    assert.match(auth.terms.intro, /HumanWill/, `${locale} provenance`);
    assert.match(auth.terms.a8Body, /HumanWill/, `${locale} version`);
    assert.match(auth.terms.a1Body, /AGPL-3\.0/, `${locale} license reference`);
    assert.equal(createHash("sha256").update(auth.terms.a7Body).digest("hex"), warrantyHashes[locale]);
  }

  assert.match(terms.en.terms.a2Body, /authorized penetration testing/);
  assert.match(terms.ko.terms.a2Body, /승인된 침투 테스트/);
  assert.match(terms.zh.terms.a2Body, /获得授权的渗透测试/);
  assert.doesNotMatch(terms.en.terms.a2Body + terms.en.terms.a3Item2, /strictly prohibited|solely for personal learning/i);
  assert.doesNotMatch(terms.ko.terms.a2Body + terms.ko.terms.a3Item2, /엄격히 금지|용도로만 제공/);
  assert.doesNotMatch(terms.zh.terms.a2Body + terms.zh.terms.a3Item2, /严禁将本软件用于任何实际的渗透测试|本软件仅供/);
});
