import { initMediaViewers } from "./media-viewer.js";

export async function loadSavedArticle(path) {
  const response = await fetch(path, { credentials: "omit", cache: "no-store", signal: AbortSignal.timeout(15000) });
  if (!response.ok) throw new Error("Report not available yet");
  const doc = new DOMParser().parseFromString(await response.text(), "text/html");
  const article = doc.getElementById("report-article");
  if (!article) throw new Error("Report not available yet");
  // Import the saved article, not its surrounding navigation or page scripts.
  article.querySelectorAll("script").forEach(node => node.remove());
  return document.importNode(article, true);
}

export function renderSavedReport(container, path, { compact = false } = {}) {
  const pending = Object.assign(document.createElement("p"), { className: "muted", textContent: "Loading the saved report…" });
  container.replaceChildren(pending);
  loadSavedArticle(path).then(article => {
    if (container.firstChild !== pending) return; // a newer refresh owns this container
    if (compact) {
      for (const heading of article.querySelectorAll("h1, h2")) {
        const smaller = document.createElement(heading.tagName === "H1" ? "h3" : "h4");
        for (const attribute of heading.attributes) smaller.setAttribute(attribute.name, attribute.value);
        smaller.append(...heading.childNodes); heading.replaceWith(smaller);
      }
    }
    container.replaceChildren(article);
    initMediaViewers(article);
  }).catch(() => {
    if (container.firstChild !== pending) return;
    pending.textContent = "The saved report is not available on this site yet. ";
    const link = Object.assign(document.createElement("a"), { href: path, textContent: "Open the report →" });
    pending.append(link);
  });
}
