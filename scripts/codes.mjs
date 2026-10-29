// Gives every report a short code, its permanent identifier: six characters of Crockford's base32
// (digits and capitals without I, L, O and U), shown as CET-7KQ2MX. It is written into the report's
// front matter as `code:` the first time, from a hash of the slug (src/lib/code.js), so it stays the same if the slug,
// title or folder later change, and sessions adding reports at the same time never need to agree on
// a number. A clash with an existing code (about one in a billion) takes the next hash.
//
// Run it before committing new reports: npm run codes. The build never writes to report files; for a
// report without one it works out the same code (src/lib/code.js).
import fs from "node:fs";
import path from "node:path";
import { codeOf } from "../src/lib/code.js";

const files = fs.readdirSync("reports", { recursive: true }).filter((f) => f.endsWith(".md")).map((f) => path.join("reports", f));
const used = new Set();
const pending = [];
for (const file of files) {
  const text = fs.readFileSync(file, "utf8");
  const m = text.match(/^---\n(?:[\s\S]*?\n)?code: ([0-9A-Z]{6})\n/);
  if (m) used.add(m[1]); else pending.push([file, text]);
}
for (const [file, text] of pending) {
  const slug = path.basename(file, ".md");
  let code, k = 0;
  do code = codeOf(k ? `${slug}#${k}` : slug), k++; while (used.has(code));
  used.add(code);
  fs.writeFileSync(file, text.replace(/^---\n/, `---\ncode: ${code}\n`));
  console.log(`${file}: CET-${code}`);
}
if (!pending.length) console.log(`all ${files.length} reports have a code`);
