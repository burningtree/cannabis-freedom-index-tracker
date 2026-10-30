import test from "node:test";
import assert from "node:assert/strict";
import { figureRows, formatWeight } from "../src/lib/figures.js";

test("cannabis weights choose units without losing small quantities", () => {
  for (const [kg, expected] of [
    [0, "0 kg"],
    [0.0000001, "0.1 mg"],
    [0.00047, "470 mg"],
    [0.000999, "999 mg"],
    [0.001, "1 g"],
    [0.00386, "3.86 g"],
    [0.09892, "98.92 g"],
    [0.999, "999 g"],
    [1, "1 kg"],
    [1.235, "1.235 kg"],
    [999.5, "999.5 kg"],
    [1000, "1 t"],
    [3565, "3.565 t"],
    [1500000, "1,500 t"],
  ]) assert.equal(formatWeight(kg), expected, `${kg} kg`);
});

test("published and draft figure rows retain the recorded gram precision", () => {
  assert.deepEqual(figureRows({ arrested: 1, kg_seized: 0.00386 }), [
    { key: "arrested", label: "Arrested", value: "1" },
    { key: "kg_seized", label: "Cannabis", value: "3.86 g" },
  ]);
  assert.ok(!figureRows({}).some(row => row.key === "kg_seized"));
  assert.equal(formatWeight(0.1 + 0.2), "300 g");
});
