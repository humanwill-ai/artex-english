import { parse, TYPE } from "@formatjs/icu-messageformat-parser";

import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

export function flatten(value, prefix = "", result = {}) {
  assert.ok(value && typeof value === "object" && !Array.isArray(value), `Invalid catalog object: ${prefix}`);
  for (const [key, item] of Object.entries(value)) {
    const name = prefix ? `${prefix}.${key}` : key;
    if (typeof item === "string") result[name] = item;
    else flatten(item, name, result);
  }
  return result;
}

// Ignore translated prose and word order, but preserve arguments, argument types,
// tag nesting, selectors, plural offsets and branch contracts. A translator may
// reorder variables within a sentence; moving a variable out of a tag or branch
// would still change this signature and fail.
export function signature(nodes) {
  return nodes
    .flatMap((node) => {
      if (node.type === TYPE.literal) return [];
      if (node.type === TYPE.tag) return [["tag", node.value, signature(node.children)]];
      if (node.options)
        return [
          [
            node.type,
            node.value,
            node.pluralType,
            node.offset,
            Object.entries(node.options)
              .sort(([a], [b]) => a.localeCompare(b))
              .map(([key, option]) => [key, signature(option.value)]),
          ],
        ];
      return [[node.type, node.value ?? null, node.style ?? null]];
    })
    .sort((a, b) => JSON.stringify(a).localeCompare(JSON.stringify(b)));
}

export function validateCatalogs(source, translated) {
  const original = flatten(source);
  const english = flatten(translated);
  assert.deepEqual(
    Object.keys(english).sort(),
    Object.keys(original).sort(),
    "English catalog keys differ from source",
  );
  for (const [key, text] of Object.entries(english)) {
    assert.ok(text.trim(), `${key}: empty English message`);
    assert.doesNotMatch(text, /[\p{Script=Hangul}\p{Script=Han}]/u, `${key}: untranslated application text`);
    const expected = signature(parse(original[key]));
    const actual = signature(parse(text));
    assert.deepEqual(actual, expected, `${key}: changed placeholders, formatting, or rich-text contract`);
  }
  return Object.keys(english).length;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const read = (name) => JSON.parse(fs.readFileSync(new URL(`../messages/${name}.json`, import.meta.url), "utf8"));
  const count = validateCatalogs(read("ko"), read("en"));
  assert.deepEqual(
    Object.keys(flatten(read("zh"))).sort(),
    Object.keys(flatten(read("en"))).sort(),
    "Chinese reference keys differ",
  );
  console.log(`English catalog: ${count} messages; key, ICU, and rich-text contracts verified.`);
}
