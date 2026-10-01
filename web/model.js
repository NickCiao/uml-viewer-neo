// model.js: page data and a folder in; boxes, merged arrows, badge counts,
// folder lamps and box sizes out. No DOM and no layout here.

const ORDER = ['unlit', 'green', 'amber', 'red'];

// Box geometry in pixels, shared with render.js. Text is monospaced, so a
// box's width follows from its character count.
export const GEOMETRY = { padX: 12, padY: 10, line: 18, lamp: 10, gap: 8, nameW: 7.8, detailW: 7.2, libH: 28, libPad: 16 };

export function worstGrade(grades) {
  let worst = 'unlit';
  for (const g of grades) if (ORDER.indexOf(g) > ORDER.indexOf(worst)) worst = g;
  return worst;
}

export function shorten(name, max = 24) {
  if (name.length <= max) return name;
  const keep = max - 1;
  const head = Math.ceil(keep / 2);
  return name.slice(0, head) + '…' + name.slice(name.length - (keep - head));
}

const keyOf = (segs) => segs.join('/');
const under = (tree, folder) => folder.every((s, i) => tree[i] === s);

export function folderExists(page, folder) {
  return folder.length === 0 || page.modules.some((m) => m.tree.length > folder.length && under(m.tree, folder));
}

function size(label, detail) {
  const g = GEOMETRY;
  const text = Math.max(label.length * g.nameW, detail.length * g.detailW);
  return { width: Math.ceil(g.padX + g.lamp + g.gap + text + g.padX), height: (detail ? 2 : 1) * g.line + 2 * g.padY };
}

export function viewAt(page, folder) {
  const depth = folder.length;
  const byId = new Map(page.modules.map((m) => [m.id, m]));
  const libs = new Map(page.libraries.map((l) => [l.id, l]));

  // The box a module belongs to at this level, or null when it is outside.
  // A folder's own module (cart/index.ts inside cart/) is a box of its own.
  const boxKey = (m) => {
    if (!under(m.tree, folder)) return null;
    if (m.tree.length === depth) return keyOf(m.tree);
    return keyOf(m.tree.slice(0, depth + 1));
  };
  // How a partner outside this folder is named: its top-level name, or at
  // depth n its first n segments.
  const outsideKey = (m) => keyOf(m.tree.slice(0, Math.max(1, depth)));

  const boxes = new Map();
  for (const m of page.modules) {
    const key = boxKey(m);
    if (key === null) continue;
    if (!boxes.has(key)) {
      boxes.set(key, { key, name: key.split('/').pop(), module: null, members: [], folder: false,
        uses: { on: new Set(), off: new Set() }, usedBy: { on: new Set(), off: new Set() } });
    }
    const b = boxes.get(key);
    b.members.push(m);
    if (keyOf(m.tree) === key) b.module = m;
    else b.folder = true;
  }

  const arrows = new Map();
  const shownLibs = new Map();
  for (const m of page.modules) {
    const from = boxKey(m);
    for (const target of m.uses) {
      if (libs.has(target)) {
        if (from === null) continue;
        shownLibs.set(target, libs.get(target));
        boxes.get(from).uses.on.add(target);
        arrows.set(`${from}\n${target}`, { from, to: target });
        continue;
      }
      const t = byId.get(target);
      if (!t) continue;
      const to = boxKey(t);
      if (from !== null && to !== null) {
        if (from === to) continue;
        boxes.get(from).uses.on.add(to);
        boxes.get(to).usedBy.on.add(from);
        arrows.set(`${from}\n${to}`, { from, to });
      } else if (from !== null) {
        boxes.get(from).uses.off.add(outsideKey(t));
      } else if (to !== null) {
        boxes.get(to).usedBy.off.add(outsideKey(m));
      }
    }
  }

  const counts = { red: 0, amber: 0, green: 0, unlit: 0 };
  const list = [...boxes.values()].sort((a, b) => a.key.localeCompare(b.key)).map((b) => {
    const grades = b.members.map((m) => m.grade);
    const measured = grades.filter((g) => g !== 'unlit').length;
    const label = shorten(b.name) + (b.folder ? '/ ›' : '');
    const n = b.members.length;
    const detail = b.folder ? `${n} module${n === 1 ? '' : 's'} · ${measured} measured` : '';
    const grade = worstGrade(grades);
    counts[grade] += 1;
    return {
      key: b.key, name: b.name, kind: b.folder ? (b.module ? 'both' : 'folder') : 'module',
      label, detail, moduleId: b.module ? b.module.id : null, grade,
      stats: b.module && !b.folder ? b.module.stats || null : null,
      modules: b.members.length, measured,
      uses: { on: [...b.uses.on].sort(), off: [...b.uses.off].sort() },
      usedBy: { on: [...b.usedBy.on].sort(), off: [...b.usedBy.off].sort() },
      ...size(label, detail),
    };
  });
  const libraries = [...shownLibs.values()].sort((a, b) => a.id.localeCompare(b.id)).map((l) => ({
    key: l.id, id: l.id, name: l.name,
    width: Math.ceil(l.name.length * GEOMETRY.detailW + 2 * GEOMETRY.libPad), height: GEOMETRY.libH,
  }));
  return { folder: [...folder], boxes: list, libraries, arrows: [...arrows.values()], counts };
}
