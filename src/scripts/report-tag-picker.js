import { rankedTags, commonTags } from "../lib/report-tags.js";

export function initTagPicker(root, { selected, onSelect }) {
  const common = root.querySelector('[data-tag-common]');
  const toggle = root.querySelector('[data-tag-toggle]');
  const panel = root.querySelector('[data-tag-panel]');
  const search = root.querySelector('[data-tag-search]');
  const extra = root.querySelector('[data-tag-extra]');
  const more = root.querySelector('[data-tag-more]');
  let ranked = [], expanded = false, limit = 48;
  const button = ({ tag, count }) => {
    const node = document.createElement('button'); node.type = 'button'; node.className = 'tag-choice'; node.dataset.tag = tag;
    node.setAttribute('aria-pressed', String(tag === selected()));
    node.title = `${count.toLocaleString('en-GB')} ${count === 1 ? 'report' : 'reports'} tagged #${tag}`;
    const label = document.createElement('span'); label.textContent = `#${tag}`;
    const number = document.createElement('small'); number.textContent = count.toLocaleString('en-GB');
    node.append(label, number); return node;
  };
  function draw() {
    root.hidden = ranked.length === 0 && !selected();
    const popular = commonTags(ranked, selected());
    common.replaceChildren(...popular.map(button));
    const used = new Set(popular.map(t => t.tag));
    const available = ranked.filter(t => !used.has(t.tag));
    toggle.hidden = available.length === 0 && !expanded;
    toggle.textContent = expanded ? 'Show less' : 'Show more';
    toggle.setAttribute('aria-expanded', String(expanded));
    panel.hidden = !expanded;
    // Thousands of local tags stay searchable without creating thousands of buttons.
    const query = search.value.trim().toLowerCase().replace(/\s+/g, '-').replace(/^#/, '');
    const matching = available.filter(t => t.tag.includes(query));
    extra.replaceChildren(...matching.slice(0, limit).map(button));
    const empty = root.querySelector('[data-tag-empty]');
    empty.hidden = matching.length > 0;
    more.hidden = matching.length <= limit;
    more.textContent = `More tags (${Math.max(0, matching.length - limit).toLocaleString('en-GB')} remaining)`;
  }
  toggle.addEventListener('click', () => { expanded = !expanded; limit = 48; draw(); if (expanded) search.focus({ preventScroll: true }); });
  search.addEventListener('input', () => { limit = 48; draw(); });
  more.addEventListener('click', () => { limit += 48; draw(); });
  root.addEventListener('click', event => {
    const node = event.target.closest('[data-tag]');
    if (!node) return;
    const tag = node.dataset.tag;
    onSelect(tag === selected() ? '' : tag);
    const next = [...root.querySelectorAll('[data-tag]')].find(n => n.dataset.tag === tag && !n.closest('[hidden]'));
    (next || toggle).focus({ preventScroll: true });
  });
  return rows => { ranked = rankedTags(rows); limit = 48; draw(); };
}
