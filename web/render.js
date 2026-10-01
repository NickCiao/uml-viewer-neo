// render.js: positions in; SVG for the diagram and HTML for the header and
// card out, as strings. No DOM here, so all of it runs under node --test.
import { GEOMETRY as G, viewAt } from './model.js';

const ESCAPES = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
export const esc = (s) => String(s ?? '').replace(/[&<>"']/g, (c) => ESCAPES[c]);
const fmt = (n) => n.toFixed(1);
const displayName = (key) => (key.startsWith('lib:') ? key.slice(4) : key);

export function badgeText(on, off) {
  if (on && off) return `${on}+${off}`;
  if (on) return `${on}`;
  if (off) return `+${off}`;
  return '';
}

const marker = (id) => `<marker id="${id}" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="7" markerHeight="7" ` +
  `markerUnits="userSpaceOnUse" orient="auto"><path class="${id}" d="M0,0 L8,4 L0,8 z"/></marker>`;
const MARKERS = marker('head') + marker('head-hi');
const MARGIN = 24;

export function renderDiagram(view, pos, ui) {
  if (view.boxes.length === 0) return '<p class="empty">Nothing to show here.</p>';
  const sel = ui.selected;
  const selBox = view.boxes.find((b) => b.key === sel) || null;
  const selLib = view.libraries.find((l) => l.key === sel) || null;
  const partners = new Set(selBox ? [...selBox.uses.on, ...selBox.usedBy.on]
    : selLib ? view.boxes.filter((b) => b.uses.on.includes(sel)).map((b) => b.key) : []);
  const nameOf = (key) => {
    const b = view.boxes.find((x) => x.key === key);
    return b ? b.name + (b.kind === 'module' ? '' : '/') : displayName(key);
  };
  const dim = (key) => sel && key !== sel && !partners.has(key);

  const arrows = pos.edges.map((e) => {
    const touches = sel && (e.from === sel || e.to === sel);
    if (!ui.arrows && !touches) return '';
    const cls = touches ? 'arrow hi' : sel ? 'arrow faded' : 'arrow';
    const d = 'M' + e.points.map(([x, y]) => `${x},${y}`).join(' L');
    return `<g class="edge"><title>${esc(nameOf(e.from))} → ${esc(nameOf(e.to))}</title>` +
      `<path class="hit" d="${d}"/><path class="${cls}" d="${d}" marker-end="url(#${touches ? 'head-hi' : 'head'})"/></g>`;
  }).join('');

  const boxes = view.boxes.map((b) => {
    const p = pos.nodes[b.key];
    const cls = ['box', b.kind === 'module' ? '' : 'folder', b.key === sel ? 'selected' : '', dim(b.key) ? 'dim' : '']
      .filter(Boolean).join(' ');
    const cy = G.padY + G.line / 2;
    const tx = G.padX + G.lamp + G.gap;
    const usedBy = badgeText(b.usedBy.on.length, b.usedBy.off.length);
    const uses = badgeText(b.uses.on.length, b.uses.off.length);
    return `<g class="${cls}" data-key="${esc(b.key)}" transform="translate(${p.x},${p.y})">` +
      `<rect width="${p.w}" height="${p.h}" rx="2"/>` +
      `<circle class="lamp lamp-${b.grade}" cx="${G.padX + G.lamp / 2}" cy="${cy}" r="${G.lamp / 2}"><title>${esc(lampText(b))}</title></circle>` +
      `<text class="name" x="${tx}" y="${cy + 4.5}"><title>${esc(b.name)}</title>${esc(b.label)}</text>` +
      (b.detail ? `<text class="detail" x="${tx}" y="${cy + G.line + 4}">${esc(b.detail)}</text>` : '') +
      (usedBy ? `<text class="badge" x="${p.w - 4}" y="-5" text-anchor="end"><title>${esc('used by: ' + partnerNames(b.usedBy))}</title>${usedBy}</text>` : '') +
      (uses ? `<text class="badge" x="${p.w - 4}" y="${p.h + 14}" text-anchor="end"><title>${esc('uses: ' + partnerNames(b.uses))}</title>${uses}</text>` : '') +
      '</g>';
  }).join('');

  const libs = view.libraries.map((l) => {
    const p = pos.nodes[l.key];
    const cls = ['lib', l.key === sel ? 'selected' : '', dim(l.key) ? 'dim' : ''].filter(Boolean).join(' ');
    return `<g class="${cls}" data-key="${esc(l.key)}" transform="translate(${p.x},${p.y})">` +
      `<ellipse cx="${p.w / 2}" cy="${p.h / 2}" rx="${p.w / 2}" ry="${p.h / 2}"/>` +
      `<text x="${p.w / 2}" y="${p.h / 2 + 4}" text-anchor="middle">${esc(l.name)}</text></g>`;
  }).join('');

  const vb = `${-MARGIN} ${-MARGIN} ${pos.width + 2 * MARGIN} ${pos.height + 2 * MARGIN}`;
  return `<svg xmlns="http://www.w3.org/2000/svg" class="diagram" viewBox="${vb}" preserveAspectRatio="xMidYMin meet">` +
    `<defs>${MARKERS}</defs><g class="edges">${arrows}</g>${boxes}${libs}</svg>`;
}

function lampText(b) {
  if (b.grade === 'unlit') return 'Not measured';
  if (b.stats) return `Grade: ${b.grade} (μ + σ = ${fmt(b.stats.mu + b.stats.sigma)})`;
  return `Worst measured: ${b.grade} (${b.measured} of ${b.modules} measured)`;
}

function partnerNames(side) {
  return [...side.on.map(displayName), ...side.off.map((k) => `${k} (outside)`)].join(', ');
}

const stamp = (iso) => (iso ? iso.replace('T', ' ').slice(0, 16) : '');

export function renderHeader(page, view, notice, ui) {
  const crumbs = [`<a data-folder="">${esc(page.repo)}</a>`].concat(view.folder.map((seg, i) =>
    `<a data-folder="${esc(view.folder.slice(0, i + 1).join('/'))}">${esc(seg)}/</a>`)).join(' ');
  const c = view.counts;
  const legend = page.coverageAt
    ? [['red', 'high'], ['amber', 'medium'], ['green', 'low'], ['unlit', 'not measured']]
        .map(([g, label]) => `<span><span class="lamp-dot lamp-${g}"></span>${c[g]} ${label}</span>`).join(' · ')
    : '<span><span class="lamp-dot lamp-unlit"></span>not measured: run <code>umlv --metrics</code></span>';
  const times = `<span class="label">scanned</span> ${esc(stamp(page.scannedAt))} · <span class="label">coverage</span> ` +
    (page.coverageAt ? esc(stamp(page.coverageAt)) : 'none yet');
  return `<nav class="crumbs">${crumbs}</nav>` +
    `<div class="legend">${legend}<span>→ imports</span>` +
    `<button data-toggle-arrows>arrows ${ui.arrows ? 'on' : 'off'}</button></div>` +
    `<div class="times">${times}</div>` +
    (notice ? `<p class="notice">${esc(notice)}</p>` : '');
}

export function renderCard(page, view, key) {
  const lib = view.libraries.find((l) => l.key === key);
  if (lib) {
    const users = view.boxes.filter((b) => b.uses.on.includes(key)).map((b) => b.key);
    return `<h2>${esc(lib.name)}</h2><p class="path">outside library</p>` + partners('Used by', { on: users, off: [] });
  }
  const b = view.boxes.find((x) => x.key === key);
  if (!b) return '';
  const m = b.moduleId ? page.modules.find((x) => x.id === b.moduleId) : null;
  let html = `<h2><span class="lamp-dot lamp-${b.grade}"></span>${esc(b.name)}${b.kind === 'module' ? '' : '/'}</h2>`;
  if (m) html += moduleSection(page, m);
  if (b.kind !== 'module') html += folderSection(page, b);
  return html + partners('Uses', b.uses) + partners('Used by', b.usedBy);
}

const link = (page, path, line) => esc(page.editorPrefix + encodeURI(path) + (line ? `:${line}` : ''));

function moduleSection(page, m) {
  let html = `<a class="path" href="${link(page, m.source)}">${esc(m.source || '.')}</a>`;
  if (!m.stats) return html + '<p>Grade: not measured</p>';
  const band = (page.bands || {})[m.grade] || '';
  html += `<p>Grade: ${m.grade} (μ + σ = ${fmt(m.stats.mu + m.stats.sigma)}; ${m.grade} is ${esc(band)})</p>`;
  html += `<p class="stats">CRAP mean ${fmt(m.stats.mu)} · spread ${fmt(m.stats.sigma)} · worst ${fmt(m.stats.max)}</p>`;
  const fns = [...m.functions].sort((a, b) => (b.crap ?? -1) - (a.crap ?? -1));
  const rows = fns.map((f, i) =>
    `<tr${i === 0 && f.crap != null ? ' class="worst"' : ''}>` +
    `<td><a href="${link(page, f.file, f.line)}">${esc(f.name)}</a></td>` +
    `<td>${f.crap != null ? fmt(f.crap) : '–'}</td><td>${f.cc}</td>` +
    `<td>${f.coverage != null ? Math.round(f.coverage * 100) + '%' : '–'}</td></tr>`).join('');
  return html + '<h3>Functions</h3><table><thead><tr><th>function</th><th>crap</th><th>cc</th><th>cov</th></tr></thead>' +
    `<tbody>${rows}</tbody></table>`;
}

function folderSection(page, b) {
  const children = viewAt(page, b.key.split('/')).boxes.filter((c) => c.key !== b.key);
  const items = children.map((c) => `<li><span class="lamp-dot lamp-${c.grade}"></span>${esc(c.label)}</li>`).join('');
  return `<p>${b.measured} of ${b.modules} measured</p><button data-open="${esc(b.key)}">Open</button>` +
    `<h3>Contents</h3><ul>${items}</ul>`;
}

function partners(title, side) {
  if (side.on.length + side.off.length === 0) return '';
  const items = side.on.map((k) => `<li>${esc(displayName(k))}</li>`).join('') +
    side.off.map((k) => `<li class="off">${esc(k)} (outside)</li>`).join('');
  return `<h3>${title}</h3><ul>${items}</ul>`;
}
