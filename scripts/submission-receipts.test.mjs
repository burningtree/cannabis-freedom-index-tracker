import { test } from "node:test";
import assert from "node:assert/strict";
import { parseReceipts, mergeReceipts, safeHTTP } from "../src/lib/submission-receipts.js";
const receipt = { submission_id: "a".repeat(32), created_at: 1793232000 };
test("receipt exports round trip and imports deduplicate", () => {
  const a = parseReceipts(JSON.parse(JSON.stringify({ version: 1, receipts: [receipt] })));
  assert.deepEqual(mergeReceipts(a, a), [receipt]);
});
test("malformed, oversized and unsupported receipt imports fail", () => {
  for (const value of [{ version: 2, receipts: [] }, { version: 1, receipts: ["bad"] }, { version: 1, receipts: [{ ...receipt, submission_id: "<script>" }] }, { version: 1, receipts: Array(501).fill(receipt) }]) assert.throws(() => parseReceipts(value));
});
test("public links cannot execute script or include credentials", () => {
  assert.equal(safeHTTP("javascript:alert(1)"), null);
  assert.equal(safeHTTP("https://secret:password@example.com/"), null);
  assert.equal(safeHTTP("https://example.com/news"), "https://example.com/news");
});
