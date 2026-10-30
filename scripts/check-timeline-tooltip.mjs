// Browser regression checks (requires npm run browser-install).
// Run explicitly: node scripts/check-timeline-tooltip.mjs
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { chromium } from "playwright";

const browser = await chromium.launch({ headless: true, channel: "chromium" });
try {
  const page = await browser.newPage({ viewport: { width: 640, height: 600 } });
  await page.clock.install();
  const css = readFileSync(new URL("../src/styles/global.css", import.meta.url), "utf8");
  await page.setContent(`<style>${css}\nbody { min-height: 1800px; } .months { width: 180px; gap: 8px; margin: 240px 20px 0; }</style>
    <div class="months" id="months">${[26, 27, 28].map((day) => `<button data-m="2026-10-${day}" data-period="day" data-period-label="${day} Oct 2026" data-total="3" data-categories='{"seizure":3}' aria-pressed="false"><span class="month-stack" style="height:100%"><i style="height:100%"></i></span></button>`).join("")}</div>`);
  const source = readFileSync(new URL("../src/scripts/report-timeline-tooltip.js", import.meta.url), "utf8");
  await page.addScriptTag({ type: "module", content: `${source}\ntimelineTooltip(document.getElementById('months'), { seizure: { label: 'Seizure', color: '#e3b341' } });` });
  const bars = page.locator("#months button");
  const tip = page.locator("#report-timeline-tooltip");
  const first = await bars.nth(0).boundingBox(), second = await bars.nth(1).boundingBox();
  const y = first.y + 20;
  await page.mouse.move(first.x + first.width / 2, y);
  assert.equal(await tip.isVisible(), true);
  // The gap produces pointerout from both the old bar and the chart itself.
  // Their dismissal timers must not hide the new bar's tooltip later.
  await page.mouse.move(first.x + first.width + 4, y);
  await page.mouse.move(second.x + second.width / 2, y);
  await page.clock.runFor(250);
  assert.equal(await tip.isVisible(), true, "crossing a gap must not leave an old hide timer");
  assert.ok((await tip.innerText()).includes("27 Oct 2026"));

  for (let i = 0; i < 5; i++) {
    await page.mouse.move(10, y);
    await page.mouse.move(second.x + second.width / 2, y);
    await page.clock.runFor(200);
    assert.equal(await tip.isVisible(), true, "quick exit/re-entry must cancel dismissal");
  }
  const bounds = await tip.boundingBox();
  await page.mouse.move(bounds.x + bounds.width / 2, bounds.y + bounds.height / 2);
  await page.clock.runFor(250);
  assert.equal(await tip.isVisible(), true, "tooltip stays open while hovered");
  await bars.nth(2).focus();
  assert.ok((await tip.innerText()).includes("28 Oct 2026"));
  await page.keyboard.press("Escape");
  assert.equal(await tip.isVisible(), false);

  await page.mouse.move(second.x + second.width / 2, y);
  await page.evaluate(() => window.dispatchEvent(new Event("scroll")));
  assert.equal(await tip.isVisible(), false);
  await page.mouse.move(second.x + second.width / 2 + 1, y);
  assert.equal(await tip.isVisible(), true, "moving within the bar after a scroll restores the tooltip");
  await page.mouse.move(10, 10);
  await page.clock.runFor(200);
  assert.equal(await tip.isVisible(), false);
  console.log("Timeline tooltips: gap crossings, rapid re-entry, tooltip hover, keyboard/Escape, scroll recovery and dismissal passed.");
} finally {
  await browser.close();
}
