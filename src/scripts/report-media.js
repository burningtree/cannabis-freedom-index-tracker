import { mediaView } from "../lib/media.js";
import { initMediaViewers } from "./media-viewer.js";

const el = (tag, text, cls) => {
  const node = document.createElement(tag);
  if (text) node.textContent = text;
  if (cls) node.className = cls;
  return node;
};
const http = (url) => { try { return /^https?:$/.test(new URL(url).protocol); } catch { return false; } };
const link = (url, text) => {
  const a = el("a", text, "ext");
  a.href = url; a.target = "_blank"; a.rel = "noopener noreferrer";
  return a;
};

// Same media types, image proxy, markup and gallery as the published report.
export function renderReportMedia(container, media, { compact = false } = {}) {
  const items = media.filter(m => http(m?.url)).map(m => ({ ...m, view: mediaView(m) }));
  if (!items.length) return;
  container.append(el(compact ? "h4" : "h2", "Photos and video"));
  const shown = items.filter(m => m.view), linked = items.filter(m => !m.view);
  if (shown.length) {
    const grid = el("div", null, "media-grid");
    grid.dataset.mediaGrid = ""; grid.dataset.count = String(Math.min(shown.length, 4));
    for (const m of shown) {
      const figure = el("figure", null, `media-tile${m.view.type === "photo" ? "" : " media-tile-video"}`);
      if (m.view.type === "photo") {
        const button = el("button", null, "media-open"); button.type = "button";
        button.setAttribute("aria-label", `Enlarge: ${m.caption}`);
        const img = el("img"); img.src = m.view.src; img.alt = m.caption || "";
        img.loading = "lazy"; img.decoding = "async"; img.referrerPolicy = "no-referrer";
        button.append(img); figure.append(button);
      } else if (m.view.type === "video") {
        const video = el("video"); video.src = m.view.src; video.controls = true;
        video.preload = "metadata"; video.playsInline = true; video.setAttribute("referrerpolicy", "no-referrer"); video.setAttribute("aria-label", m.caption || "Video");
        figure.append(video);
      } else {
        const frame = el("iframe"); frame.src = m.view.src; frame.title = m.caption || "Video";
        frame.loading = "lazy"; frame.allow = "fullscreen; picture-in-picture"; frame.allowFullscreen = true;
        frame.referrerPolicy = "strict-origin-when-cross-origin"; figure.append(frame);
      }
      const caption = el("figcaption", m.caption);
      const source = el("span", "Source: ", "media-source"); source.append(link(m.url, m.publisher || m.url));
      if (http(m.archived)) source.append(document.createTextNode(" · "), link(m.archived, "archived copy"));
      caption.append(source); figure.append(caption); grid.append(figure);
    }
    container.append(grid);
  }
  if (linked.length) {
    const list = el("ul", null, "media");
    for (const m of linked) {
      const li = el("li"); li.append(link(m.url, m.caption || m.url), el("span", ` ${m.kind === "video" ? "Video" : "Photos"}: ${m.publisher || ""}`, "muted"));
      if (http(m.archived)) li.append(document.createTextNode(" · "), link(m.archived, "archived copy"));
      list.append(li);
    }
    container.append(list);
  }
  initMediaViewers(container);
}
