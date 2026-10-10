import { validateCatalogs } from "../../scripts/check-i18n.mjs";
import assert from "node:assert/strict";
import test from "node:test";

test("catalog validation accepts translated prose and argument reordering", () => {
  assert.equal(validateCatalogs({ x: "{a} 와 {b}" }, { x: "{b} and {a}" }), 1);
});

test("catalog validation rejects missing keys, arguments, invalid ICU and moved rich-text variables", () => {
  assert.throws(() => validateCatalogs({ x: "test" }, {}));
  assert.throws(() => validateCatalogs({ x: "{count}" }, { x: "Count" }));
  assert.throws(() => validateCatalogs({ x: "{count}" }, { x: "{count" }));
  assert.throws(() => validateCatalogs({ x: "<b>{name}</b>" }, { x: "<b>Name</b> {name}" }));
  assert.throws(() => validateCatalogs({ x: "{count, plural, one {1} other {#}}" }, { x: "{count, number}" }));
  assert.throws(() => validateCatalogs({ x: "저장" }, { x: "저장" }));
  assert.throws(() => validateCatalogs({ x: "저장" }, { x: "" }));
});
