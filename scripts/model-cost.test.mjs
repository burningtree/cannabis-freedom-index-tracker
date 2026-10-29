import test from "node:test";
import assert from "node:assert/strict";
import { runCost, isPeak, costEstimated, formatUSD } from "../src/lib/model-cost.js";

const model = "opencode-go/deepseek-v4.1-flash";
// 26 October 2026 is a Monday; 12:00 UTC is off-peak
const offPeak = Date.UTC(2026, 9, 26, 12) / 1000;
const peak = Date.UTC(2026, 9, 26, 2) / 1000;

test("a million tokens each way cost the published price off-peak", () => {
  assert.equal(runCost({ model, input_tokens: 1_000_000, output_tokens: 1_000_000, started_at: offPeak }), 0.75);
});

test("peak hours cost double", () => {
  assert.equal(runCost({ model, input_tokens: 1_000_000, output_tokens: 1_000_000, started_at: peak }), 1.5);
});

test("peak hours are 01:00-04:00 and 06:00-10:00 UTC on weekdays", () => {
  assert.equal(isPeak(Date.UTC(2026, 9, 26, 1) / 1000), true);
  assert.equal(isPeak(Date.UTC(2026, 9, 26, 3, 59) / 1000), true);
  assert.equal(isPeak(Date.UTC(2026, 9, 26, 4) / 1000), false);
  assert.equal(isPeak(Date.UTC(2026, 9, 26, 6) / 1000), true);
  assert.equal(isPeak(Date.UTC(2026, 9, 26, 10) / 1000), false);
  // Saturday and Sunday are off-peak all day
  assert.equal(isPeak(Date.UTC(2026, 9, 31, 2) / 1000), false);
  assert.equal(isPeak(Date.UTC(2026, 10, 1, 2) / 1000), false);
});

test("a reported cost wins over the token estimate", () => {
  assert.equal(runCost({ model, cost_usd: 0.02, input_tokens: 1_000_000, output_tokens: 1_000_000 }), 0.02);
  assert.equal(costEstimated({ model, cost_usd: 0.02 }), false);
  assert.equal(costEstimated({ model }), true);
});

test("an unknown model or no tokens has no cost", () => {
  assert.equal(runCost({ model: "other/model", input_tokens: 1_000_000, output_tokens: 1_000_000 }), 0);
  assert.equal(runCost({ model }), 0);
  assert.equal(costEstimated({ model: "other/model" }), false);
});

test("the example run is a fraction of a cent off-peak", () => {
  const cost = runCost({ model, input_tokens: 39_521, output_tokens: 1_619, started_at: offPeak });
  assert.ok(Math.abs(cost - 0.00689955) < 1e-9);
});

test("costs keep four decimals, and six below a ten-thousandth of a dollar", () => {
  assert.equal(formatUSD(0.0137991), "$0.0138");
  assert.equal(formatUSD(0.00689955), "$0.0069");
  assert.equal(formatUSD(1.23456), "$1.2346");
  assert.equal(formatUSD(0.0001), "$0.0001");
  assert.equal(formatUSD(0.000069), "$0.000069");
  assert.equal(formatUSD(0), "—");
});
