import { execFileSync } from "node:child_process";
import { repositoryURL } from "./changelog.js";

// One Git call for all report pages. Use commit time, never checkout/build time.
export function readReportHistory(cwd = process.cwd()) {
  const history = new Map();
  let output;
  try {
    output = execFileSync("git", ["log", "--first-parent", "--root", "--diff-merges=first-parent", "--no-renames", "--name-only", "-z", "--format=%x00REPORT_COMMIT%x00%H%x00%cI%x00", "--", "reports"], {
      cwd, encoding: "utf8", maxBuffer: 32 * 1024 * 1024, stdio: ["ignore", "pipe", "ignore"],
    });
  } catch { return history; } // no Git history: do not invent an update timestamp
  const tokens = output.split("\0");
  let hash, date;
  for (let i = 0; i < tokens.length; i++) {
    const token = tokens[i].replace(/^\n+/, "");
    if (token === "REPORT_COMMIT") {
      hash = tokens[++i]; date = tokens[++i];
    } else if (hash && token.startsWith("reports/") && token.endsWith(".md") && !history.has(token)) {
      history.set(token, {
        date,
        url: `${repositoryURL}/blob/${hash}/${token.split("/").map(encodeURIComponent).join("/")}`,
      });
    }
  }
  return history;
}
