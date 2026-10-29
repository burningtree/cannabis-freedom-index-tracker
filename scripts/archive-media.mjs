// Keeps a local copy of the pictures on each report's source and media pages, as evidence in case
// the pages change or disappear. The copies go to archive/<slug>/ with a manifest (page, image URL,
// date fetched, SHA-256); archive/ is not committed and not published, since most of these pictures
// belong to their publishers. Back it up privately.
//
//   node scripts/archive-media.mjs              every report
//   node scripts/archive-media.mjs <slug> …     only these
import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import * as yaml from "js-yaml";

const UA = "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126 Safari/537.36";
const only = process.argv.slice(2);
const files = fs.readdirSync("reports", { recursive: true }).filter((f) => f.endsWith(".md"));

// the pictures of an article: its preview image, then <img> in the page that look like photos
function imagesOf(html, page) {
  const urls = new Set();
  const abs = (u) => { try { return new URL(u.replace(/&amp;/g, "&"), page).href; } catch { return null; } };
  for (const m of html.matchAll(/<meta[^>]+(?:property|name)="(?:og:image|twitter:image)(?::src)?"[^>]+content="([^"]+)"/gi)) urls.add(abs(m[1]));
  for (const m of html.matchAll(/<meta[^>]+content="([^"]+)"[^>]+(?:property|name)="(?:og:image|twitter:image)"/gi)) urls.add(abs(m[1]));
  for (const m of html.matchAll(/<img[^>]+(?:data-src|src)="([^"]+\.(?:jpe?g|png|webp)[^"]*)"/gi)) {
    if (!/logo|icon|avatar|sprite|banner|ads?[/_-]|pixel|placeholder/i.test(m[1])) urls.add(abs(m[1]));
  }
  return [...urls].filter(Boolean);
}

for (const f of files) {
  const slug = path.basename(f, ".md");
  if (only.length && !only.includes(slug)) continue;
  const data = yaml.load(fs.readFileSync(path.join("reports", f), "utf8").match(/^---\n([\s\S]*?)\n---/)[1], { schema: yaml.JSON_SCHEMA });
  const pages = [...new Set([...data.sources.map((s) => s.url), ...(data.media ?? []).map((m) => m.url)])];
  const dir = path.join("archive", slug);
  fs.mkdirSync(dir, { recursive: true });
  const manifestFile = path.join(dir, "manifest.json");
  const manifest = fs.existsSync(manifestFile) ? JSON.parse(fs.readFileSync(manifestFile, "utf8")) : [];
  const have = new Set(manifest.map((m) => m.image));
  for (const page of pages) {
    let html;
    try { html = await (await fetch(page, { headers: { "User-Agent": UA } })).text(); } catch (e) { console.log(`${slug}: cannot open ${page} (${e.message})`); continue; }
    for (const image of imagesOf(html, page).slice(0, 12)) {
      if (have.has(image)) continue;
      try {
        const res = await fetch(image, { headers: { "User-Agent": UA, Referer: page } });
        const type = res.headers.get("content-type") ?? "";
        if (!res.ok || !type.startsWith("image/")) continue;
        const buf = Buffer.from(await res.arrayBuffer());
        if (buf.length < 15000) continue; // icons and thumbnails
        const sha256 = crypto.createHash("sha256").update(buf).digest("hex");
        if (manifest.some((m) => m.sha256 === sha256)) continue;
        const file = `${sha256.slice(0, 12)}.${type.split("/")[1].replace("jpeg", "jpg").split(";")[0]}`;
        fs.writeFileSync(path.join(dir, file), buf);
        manifest.push({ file, image, page, fetched: new Date().toISOString(), bytes: buf.length, sha256 });
        have.add(image);
      } catch { /* skip */ }
    }
  }
  fs.writeFileSync(manifestFile, JSON.stringify(manifest, null, 1) + "\n");
  console.log(`${slug}: ${manifest.length} pictures in ${dir}`);
}
