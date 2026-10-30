import { isReportDay } from "./report-timeline.js";

const presets = new Set(["7d", "30d", "1y", "all"]);

// Calendar links take precedence over legacy rolling-window parameters. Never
// intersect a chosen calendar date with an invisible recent-period restriction.
export function readPeriod(params, defaultRange = "all", { allowRelative = true } = {}) {
  const date = isReportDay(params.get("date")) ? params.get("date") : "";
  const year = date ? date.slice(0, 4) : /^\d{4}$/.test(params.get("year") || "") ? params.get("year") : "";
  const month = date ? date.slice(5, 7) : /^(0[1-9]|1[0-2])$/.test(params.get("month") || "") ? params.get("month") : "";
  const calendar = !!(year || month || date || params.get("range") === "calendar");
  const requested = params.get("range");
  const mode = calendar ? "calendar" : !allowRelative ? "all" : presets.has(requested) ? requested : defaultRange;
  return { mode, range: calendar ? "all" : mode, year, month, date };
}

export function changePeriod(mode, current, defaultYear = "") {
  if (mode === "calendar") return { mode, range: "all", year: current.year || defaultYear, month: current.month || "", date: current.date || "" };
  const range = presets.has(mode) ? mode : "all";
  return { mode: range, range, year: "", month: "", date: "" };
}
