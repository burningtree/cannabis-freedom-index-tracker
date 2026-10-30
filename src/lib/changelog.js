// Build-time history only. First-parent diffs describe each change as it reached this branch,
// including merged work once. Read blobs in one batch, never from the working tree.
import { execFileSync } from "node:child_process";
import { load } from "js-yaml";

export const repositoryURL = "https://github.com/burningtree/cannabis-freedom-index-tracker";
const isReport = (path) => path.startsWith("reports/") && path.endsWith(".md");
const slugOf = (path) => path.split("/").at(-1).replace(/\.md$/, "");
const git = (args, options = {}) => execFileSync("git", args, { maxBuffer: 128 * 1024 * 1024, ...options });

export function readChangelog(cwd = process.cwd()) {
  const run = (args, options) => git(args, { cwd, ...options });
  const shallow = run(["rev-parse", "--is-shallow-repository"], { encoding: "utf8" }).trim() === "true";
  const tokens = run(["log", "--first-parent", "--root", "--diff-merges=first-parent", "--find-renames", "--raw", "--no-abbrev", "-z", "--format=%x00COMMIT%x00%H%x00%cI%x00%s%x00"], { encoding: "utf8" }).split("\0");
  const commits = [];
  const blobs = new Set();
  let commit;
  for (let i = 0; i < tokens.length; i++) {
    const token = tokens[i].trim();
    if (token === "COMMIT") {
      commit = { hash: tokens[++i], date: tokens[++i], subject: tokens[++i], changes: [], otherFiles: 0 };
      commits.push(commit);
    } else if (token.startsWith(":")) {
      const [, , oldBlob, newBlob, status] = token.slice(1).split(" ");
      const oldPath = tokens[++i];
      const newPath = /^[RC]/.test(status) ? tokens[++i] : oldPath;
      if (!isReport(oldPath) && !isReport(newPath)) { commit.otherFiles++; continue; }
      const kind = !isReport(newPath) || status === "D" ? "deleted"
        : !isReport(oldPath) || status === "A" || status.startsWith("C") ? "added"
        : status.startsWith("R") ? "renamed" : "edited";
      const change = { kind, oldPath, path: newPath, oldBlob, newBlob };
      commit.changes.push(change);
      for (const blob of [oldBlob, newBlob]) if (!/^0+$/.test(blob)) blobs.add(blob);
    }
  }
  const snapshots = new Map();
  if (blobs.size) {
    const output = run(["cat-file", "--batch"], { input: [...blobs].join("\n") + "\n" });
    let offset = 0;
    while (offset < output.length) {
      const end = output.indexOf(10, offset);
      const [hash, , size] = output.subarray(offset, end).toString().split(" ");
      const text = output.subarray(end + 1, end + 1 + Number(size)).toString();
      const match = text.match(/^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)([\s\S]*)/);
      let data = {};
      try { data = match ? load(match[1]) ?? {} : {}; } catch { /* older invalid YAML still has a file link */ }
      snapshots.set(hash, { data, body: match?.[2] ?? text });
      offset = end + 1 + Number(size) + 1;
    }
  }
  for (const commit of commits) for (const change of commit.changes) {
    const before = snapshots.get(change.oldBlob), after = snapshots.get(change.newBlob);
    change.title = (change.kind === "deleted" ? before : after)?.data.title || slugOf(change.path);
    const snapshot = change.kind === "deleted" ? before : after;
    change.description = typeof snapshot?.data.summary === "string" ? snapshot.data.summary.trim() : "";
    change.country = typeof snapshot?.data.country === "string" ? snapshot.data.country : null;
    change.categories = [snapshot?.data.type, ...(Array.isArray(snapshot?.data.also) ? snapshot.data.also : [])].filter((value) => typeof value === "string");
    const eventDate = snapshot?.data.date;
    change.date = eventDate instanceof Date && !Number.isNaN(eventDate.valueOf()) ? eventDate.toISOString().slice(0, 10)
      : typeof eventDate === "string" && /^\d{4}-\d{2}-\d{2}$/.test(eventDate) ? eventDate : null;
    change.slug = slugOf(change.path);
    change.fields = [];
    if (before && after) {
      for (const key of new Set([...Object.keys(before.data), ...Object.keys(after.data)])) {
        if (JSON.stringify(before.data[key]) !== JSON.stringify(after.data[key])) change.fields.push(key.replaceAll("_", " "));
      }
      if (before.body.trim() !== after.body.trim()) change.fields.push("report text");
    }
    delete change.oldBlob;
    delete change.newBlob;
  }
  return { commits, shallow };
}

// Limit both commit headings and report rows, including a commit importing thousands of reports.
export function paginateChangelog(commits, size = 50) {
  const pages = [];
  let page = [], used = 0;
  for (const commit of commits) {
    const rows = commit.changes.length ? commit.changes : [null];
    for (let offset = 0; offset < rows.length;) {
      if (used === size) { pages.push(page); page = []; used = 0; }
      const chunk = rows.slice(offset, offset + size - used);
      page.push({ ...commit, changes: chunk.filter(Boolean), continued: offset > 0 });
      offset += chunk.length;
      used += chunk.length;
    }
  }
  if (page.length || !pages.length) pages.push(page);
  return pages;
}

// Commit-time activity, with empty intervals included and at most 61 bars across the history.
export function changelogTimeline(commits) {
  if (!commits.length) return { buckets: [], interval: "day" };
  const times = commits.map((commit) => new Date(commit.date).valueOf());
  const start = Math.min(...times), end = Math.max(...times), day = 86400000;
  const step = end - start < 48 * 3600000 ? 3600000
    : Math.ceil((end - start) / day) <= 60 ? day : Math.ceil((end - start) / day / 60) * day;
  const first = Math.floor(start / step) * step;
  const buckets = Array.from({ length: Math.floor((end - first) / step) + 1 }, (_, i) => ({
    date: new Date(first + i * step).toISOString(),
    end: new Date(first + (i + 1) * step).toISOString(),
    added: 0, edited: 0, deleted: 0, renamed: 0, commits: 0, hash: null,
  }));
  for (let i = 0; i < commits.length; i++) {
    const bucket = buckets[Math.floor((times[i] - first) / step)];
    bucket.commits++;
    bucket.hash ??= commits[i].hash;
    for (const change of commits[i].changes) bucket[change.kind]++;
  }
  return { buckets, interval: step === 3600000 ? "hour" : step === day ? "day" : `${step / day}-day interval` };
}
