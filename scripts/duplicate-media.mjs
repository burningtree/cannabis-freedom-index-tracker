// Finds photos a report lists twice: the same picture from two outlets (an agency handout
// republished, resized or recompressed), by a fingerprint of each photo. The submission service
// leaves these out of new reports (backend/internal/research/media_dedupe.go); this finds them in
// existing ones.
//
//   node scripts/duplicate-media.mjs              list the duplicates in every report
//   node scripts/duplicate-media.mjs <slug> …     only these
//   node scripts/duplicate-media.mjs --fix        also remove the later copy from the report file
import fs from "node:fs";
import path from "node:path";
import * as yaml from "js-yaml";

const UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126 Safari/537.36";
export const SAME = 10; // differing bits of 64, as in the service

// a difference hash: the photo shrunk to 9×8 grey cells, one bit per pair of neighbours
export async function fingerprint(buffer) {
  const sharp = (await import("sharp")).default;
  const px = await sharp(buffer).greyscale().resize(9, 8, { fit: "fill" }).raw().toBuffer();
  let hash = 0n;
  for (let y = 0; y < 8; y++) for (let x = 0; x < 8; x++) hash = (hash << 1n) | (px[y * 9 + x] > px[y * 9 + x + 1] ? 1n : 0n);
  return hash;
}
export const distance = (a, b) => { let d = 0, v = a ^ b; while (v) { d += Number(v & 1n); v >>= 1n; } return d; };

// the report text without the media entry for this address, the rest of the file untouched
export function removeMedia(text, mediaUrl) {
  const lines = text.split("\n");
  const at = lines.findIndex((l) => /^\s*-?\s*media_url:/.test(l) && l.split("media_url:")[1].trim().replace(/^["']|["']$/g, "") === mediaUrl);
  if (at < 0) return text;
  let start = at;
  while (start > 0 && !/^\s*- /.test(lines[start])) start--;
  const indent = lines[start].match(/^\s*/)[0].length;
  let end = start + 1;
  while (end < lines.length && lines[end].trim() !== "---" && !(lines[end].match(/^\s*/)[0].length <= indent && lines[end].trim() !== "") ) end++;
  lines.splice(start, end - start);
  return lines.join("\n");
}

async function main() {
  const args = process.argv.slice(2), fix = args.includes("--fix"), only = args.filter((a) => a !== "--fix");
  const files = fs.readdirSync("reports", { recursive: true }).filter((f) => f.endsWith(".md")).map((f) => path.join("reports", f));
  let found = 0;
  for (const file of files) {
    const slug = path.basename(file, ".md");
    if (only.length && !only.includes(slug)) continue;
    let text = fs.readFileSync(file, "utf8");
    const media = (yaml.load(text.split("---")[1]) ?? {}).media ?? [];
    const photos = media.filter((m) => m.kind === "photo" && m.media_url);
    if (photos.length < 2) continue;
    const seen = [];
    for (const m of photos) {
      let hash;
      try {
        const res = await fetch(m.media_url, { headers: { "User-Agent": UA }, signal: AbortSignal.timeout(20000) });
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        hash = await fingerprint(Buffer.from(await res.arrayBuffer()));
      } catch (error) { console.warn(`${slug}: could not check ${m.media_url} (${error.message})`); continue; }
      const same = seen.find((s) => distance(s.hash, hash) <= SAME);
      if (!same) { seen.push({ hash, m }); continue; }
      found++;
      console.log(`${slug}: “${m.caption}” (${m.publisher}) repeats “${same.m.caption}” (${same.m.publisher})`);
      if (fix) text = removeMedia(text, m.media_url);
    }
    if (fix) fs.writeFileSync(file, text);
  }
  console.log(found ? `${found} duplicate photo${found === 1 ? "" : "s"}${fix ? " removed" : "; run with --fix to remove them"}.` : "No duplicate photos.");
}
if (import.meta.url === `file://${process.argv[1]}`) await main();
