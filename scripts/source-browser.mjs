// Invoked by the Go source loader. Browser traffic is fulfilled only through its
// public-IP-checked fetch bridge; no account, saved cookies or provider keys load.
import { chromium } from "playwright";
import fs from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

export function extractSourcePage() {
  const clean = (value, max = 2000) => String(value || "").replace(/\s+/g, " ").trim().slice(0, max);
  const tags = {};
  for (const element of [...document.querySelectorAll("meta[name], meta[property]")].slice(0, 160)) {
    const key = (element.getAttribute("property") || element.getAttribute("name") || "").toLowerCase();
    if (key && key.length < 100 && !/(?:^|[-_:])(token|secret|password|authorization|csrf)(?:$|[-_:])|api[_-]?key/i.test(key) && !Object.hasOwn(tags, key)) tags[key] = clean(element.getAttribute("content"));
  }
  const jsonLD = [];
  for (const element of [...document.querySelectorAll('script[type="application/ld+json"]')].slice(0, 8)) {
    if (element.textContent.length > 32000) continue;
    try { jsonLD.push(JSON.parse(element.textContent)); } catch { /* preserve only valid structured data */ }
  }
  let article = {};
  const visit = (value, depth = 0) => {
    if (!value || typeof value !== "object" || depth > 6) return;
    if (Array.isArray(value)) { value.slice(0, 50).forEach(item => visit(item, depth + 1)); return; }
    const types = [].concat(value["@type"] || []);
    if (types.some(type => /^(NewsArticle|Article|BlogPosting|ReportageNewsArticle)$/.test(type)) && !article["@type"]) article = value;
    if (value["@graph"]) visit(value["@graph"], depth + 1);
  };
  jsonLD.forEach(value => visit(value));
  const authors = [].concat(article.author || []).map(author => clean(typeof author === "string" ? author : author?.name, 200)).filter(Boolean);
  if (tags.author && !authors.includes(tags.author)) authors.unshift(tags.author);
  let canonical = document.querySelector('link[rel~="canonical"]')?.getAttribute("href") || "";
  try { canonical = new URL(canonical, document.baseURI).href; } catch { canonical = ""; }
  if (!document.querySelector('link[rel~="canonical"]')) canonical = "";
  const metadata = {
    meta: tags, keywords: clean(tags.keywords),
    title: clean(document.title || tags["og:title"] || article.headline, 500),
    description: clean(tags.description || tags["og:description"] || tags["twitter:description"] || article.description),
    canonical, language: clean(document.documentElement.lang || tags["og:locale"] || article.inLanguage, 40),
    authors: authors.slice(0, 20), published_at: clean(tags["article:published_time"] || tags.date || tags.pubdate || tags.datepublished || tags["dcterms.created"] || article.datePublished, 100),
    modified_at: clean(tags["article:modified_time"] || article.dateModified, 100),
    site_name: clean(tags["og:site_name"] || tags["application-name"] || article.publisher?.name, 200), robots: clean(tags.robots, 500),
    open_graph: Object.fromEntries(Object.entries(tags).filter(([key]) => key.startsWith("og:") || key.startsWith("article:"))),
    twitter: Object.fromEntries(Object.entries(tags).filter(([key]) => key.startsWith("twitter:"))), json_ld: jsonLD,
  };
  const candidates = [document.querySelector("article"), document.querySelector("main"), document.body];
  const content = candidates.find(element => (element?.innerText || "").trim().length >= 200) || document.body;
  const text = (content?.innerText || "").trim();
  const clone = document.documentElement.cloneNode(true);
  clone.querySelectorAll("script, style, noscript, svg").forEach(element => element.remove());
  return { text, content: "<!doctype html>\n" + clone.outerHTML, metadata, height: document.documentElement.scrollHeight };
}

export async function captureSource({ url, dir, bridge, token }) {
  if (!bridge || !token || !/^http:\/\/127\.0\.0\.1:\d+$/.test(bridge)) throw new Error("Source fetch bridge is unavailable.");
  const browser = await chromium.launch({ channel: "chromium", headless: true, timeout: 12000, chromiumSandbox: true,
    proxy: { server: "http://127.0.0.1:1" },
    args: ["--proxy-bypass-list=<-loopback>", "--disable-background-networking", "--disable-quic", "--force-webrtc-ip-handling-policy=disable_non_proxied_udp"],
    env: { PATH: process.env.PATH || "", HOME: process.env.HOME || "", TMPDIR: process.env.TMPDIR || "" },
  });
  try {
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, serviceWorkers: "block", acceptDownloads: false, ignoreHTTPSErrors: false });
    await context.routeWebSocket("**/*", socket => socket.close());
    await context.route("**/*", async route => {
      const request = route.request();
      if (!/^https?:\/\//i.test(request.url()) || request.resourceType() === "media") { await route.abort(); return; }
      try {
        const response = await fetch(`${bridge}/fetch`, { method: "POST", headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
          body: JSON.stringify({ url: request.url(), method: request.method(), headers: await request.allHeaders(), body: request.postDataBuffer()?.toString("base64") || "" }), signal: AbortSignal.timeout(30000),
        });
        if (!response.ok) { await route.abort(); return; }
        const result = await response.json();
        await route.fulfill({ status: result.status, headers: result.headers, body: Buffer.from(result.body || "", "base64") });
      } catch { await route.abort().catch(() => {}); }
    });
    const page = await context.newPage();
    page.on("dialog", dialog => dialog.dismiss());
    context.on("page", popup => { if (popup !== page) popup.close().catch(() => {}); });
    let response, navigationError = "";
    try { response = await page.goto(url, { waitUntil: "domcontentloaded", timeout: 25000 }); }
    catch (error) { navigationError = String(error.message).split("\n")[0].slice(0, 300); }
    await page.waitForLoadState("networkidle", { timeout: 5000 }).catch(() => {});
    await page.waitForFunction(() => (document.body?.innerText || "").trim().length >= 200, null, { timeout: 6000 }).catch(() => {});
    // Sample text until it settles, bounded independently of long polling/analytics.
    let previous = "";
    for (let i = 0; i < 4; i++) {
      const current = await page.locator("body").innerText({ timeout: 2000 }).catch(() => "");
      if (current && current === previous) break;
      previous = current; await page.waitForTimeout(400);
    }
    const extracted = await page.evaluate(extractSourcePage);
    if (Buffer.byteLength(extracted.text) > 2 * 1024 * 1024 || Buffer.byteLength(extracted.content) > 2 * 1024 * 1024) throw new Error("The rendered source is larger than 2 MB.");
    const height = Math.max(900, Math.min(extracted.height, 12000));
    let screenshot = "", screenshotError = "";
    try { await page.screenshot({ path: path.join(dir, "page.png"), clip: { x: 0, y: 0, width: 1440, height }, animations: "disabled", timeout: 10000 }); await fs.chmod(path.join(dir, "page.png"), 0o600); screenshot = "page.png"; }
    catch { screenshotError = "The page rendered, but its screenshot could not be captured."; }
    await Promise.all([
      fs.writeFile(path.join(dir, "page.html"), extracted.content, { mode: 0o600 }),
      fs.writeFile(path.join(dir, "page.txt"), extracted.text, { mode: 0o600 }),
      fs.writeFile(path.join(dir, "metadata.json"), JSON.stringify(extracted.metadata, null, 2), { mode: 0o600 }),
    ]);
    const status = response?.status() || 0;
    const evidence = { url, final_url: page.url(), content: extracted.content, text: extracted.text, metadata: extracted.metadata, rendered: true, screenshot, browser_warning: screenshotError };
    if (height < extracted.height) evidence.browser_warning = [screenshotError, "Screenshot height limited to 12,000 pixels."].filter(Boolean).join(" ");
    const error = status >= 400 ? `The browser page answered HTTP ${status}.` : navigationError && !extracted.text ? "Browser navigation failed." : "";
    return { evidence, error, status };
  } finally { await browser.close(); }
}

async function main() {
  const [url, dir] = process.argv.slice(2);
  try { const result = await captureSource({ url, dir, bridge: process.env.SOURCE_FETCH_BRIDGE, token: process.env.SOURCE_FETCH_TOKEN }); process.stdout.write(JSON.stringify(result)); }
  catch (error) {
    const missing = /Executable doesn't exist|browserType.launch|playwright install/.test(String(error));
    process.stdout.write(JSON.stringify({ error: missing ? "Playwright Chromium is unavailable. Run npm run browser-install." : String(error.message).split("\n")[0].slice(0, 300), unavailable: missing })); process.exitCode = 1;
  }
}
if (process.argv[1] && pathToFileURL(process.argv[1]).href === import.meta.url) main();
