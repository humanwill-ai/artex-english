import { isAlreadyExistsError, isArchiveNotFoundError } from "./legacy-errors.ts";
import assert from "node:assert/strict";
import test from "node:test";

test("skill overwrite remains available with English and legacy server errors", () => {
  for (const message of ["Skill demo already exists", "demo 이미 존재", "技能已存在"])
    assert.equal(isAlreadyExistsError(message), true);
  assert.equal(isAlreadyExistsError("Access denied"), false);
});

test("archive restore recognizes disappearance without treating other errors as success", () => {
  for (const message of ["Archive not found", "보관을 찾을 수 없습니다", "归档不存在"])
    assert.equal(isArchiveNotFoundError(message), true);
  for (const message of ["Task not found", "Archive restore failed", "Unauthorized", "Network error"])
    assert.equal(isArchiveNotFoundError(message), false);
});
