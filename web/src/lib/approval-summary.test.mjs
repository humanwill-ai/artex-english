import { approvalSummary } from "./approval-summary.ts";
import assert from "node:assert/strict";
import test from "node:test";

test("approval cards retain tool and record IDs across new and historical languages", () => {
  for (const summary of [
    "Approval requested for tool Bash (#42)",
    "도구 Bash 승인 요청 (#42)",
    "工具 Bash 请求审批 (#42)",
  ]) {
    assert.deepEqual(approvalSummary(summary), { pendingId: 42, toolName: "Bash" });
  }
  assert.deepEqual(approvalSummary("Custom legacy notice"), { pendingId: null, toolName: "Custom legacy notice" });
});
