// Reports by year with a sticky month index (src/components/ReportYears.astro draws it in the page):
// the same markup drawn in the browser for the filtered Reports list, and the index's marker for the
// month being read.

// a year is split into months from this many reports, when they fall in more than one month
export const MONTHS_FROM = 4;
export const monthLabel = (month) => new Date(`${month}-01T00:00:00Z`).toLocaleDateString("en-GB", { month: "long", timeZone: "UTC" });
export const plural = (n, word) => `${n.toLocaleString("en-GB")} ${word}${n === 1 ? "" : "s"}`;

// rows: what is drawn, newest first; all: every row the filters let through (the counts), each with
// a date `d` ("2026-10-27"). item: a row's HTML.
export function yearsHTML(rows, all, item, base = "/") {
  const total = new Map();
  for (const r of all) { const m = r.d.slice(0, 7); total.set(m, (total.get(m) ?? 0) + 1); }
  const groups = new Map();
  for (const r of rows) {
    const y = r.d.slice(0, 4), m = r.d.slice(0, 7);
    if (!groups.has(y)) groups.set(y, new Map());
    const months = groups.get(y);
    if (!months.has(m)) months.set(m, []);
    months.get(m).push(r);
  }
  return [...groups].map(([year, drawn]) => {
    // the index lists every month of the year that has reports, drawn yet or not
    const months = [...total].filter(([m]) => m.startsWith(`${year}-`)).sort((a, b) => b[0].localeCompare(a[0]));
    const count = months.reduce((n, [, c]) => n + c, 0);
    const index = count >= MONTHS_FROM && months.length > 1
      ? `<ol class="ry-months" aria-label="Months of ${year}">${months.map(([m, n]) => `<li><a href="#m${m}">${monthLabel(m)}<small>${n}</small></a></li>`).join("")}</ol>` : "";
    return `<section class="ry" id="y${year}" data-year="${year}"><div class="ry-side"><h2><a href="${base}${year}/" title="All reports of ${year}">${year}</a><small>${plural(count, "report")}</small></h2>${index}</div>
      <ol class="ry-list">${[...drawn].map(([m, list]) => `<li class="ry-month" id="m${m}"><ul class="reports">${list.map(item).join("")}</ul></li>`).join("")}</ol></section>`;
  }).join("");
}

// The month index beside a year marks the month being read: the last one whose start has passed
// the top of the screen. Works on lists drawn later too.
let watching = false;
export function watchMonths() {
  if (watching) return;
  watching = true;
  let queued = false;
  const mark = () => {
    queued = false;
    for (const year of document.querySelectorAll(".ry")) {
      const nav = year.querySelector(".ry-months");
      if (!nav) continue;
      const r = year.getBoundingClientRect();
      if (r.bottom < 0 || r.top > innerHeight) continue;
      const months = [...year.querySelectorAll(".ry-month")];
      const current = months.filter((m) => m.getBoundingClientRect().top <= 160).at(-1) ?? months[0];
      for (const a of nav.querySelectorAll("a")) a.toggleAttribute("aria-current", a.hash === `#${current?.id}`);
    }
  };
  addEventListener("scroll", () => { if (!queued) { queued = true; requestAnimationFrame(mark); } }, { passive: true });
  document.addEventListener("reports:drawn", mark);
  mark();
}
