// app.js: the page's state and events. Everything it draws comes from
// model, layout and render; the pure helpers below are what the tests check.
import { viewAt, folderExists } from './model.js';
import { layout } from './layout.js';
import { renderDiagram, renderHeader, renderCard } from './render.js';

export function parseHash(hash) {
  const p = new URLSearchParams(String(hash || '').replace(/^#/, ''));
  const f = p.get('f') || '';
  return { folder: f ? f.split('/') : [], selected: p.get('s') || null, arrows: p.get('a') !== 'off' };
}

export function formatHash(state) {
  const p = new URLSearchParams();
  if (state.folder.length) p.set('f', state.folder.join('/'));
  if (state.selected) p.set('s', state.selected);
  if (!state.arrows) p.set('a', 'off');
  const s = p.toString();
  return s ? `#${s}` : '';
}

export function onEscape(state) {
  if (state.selected) return { ...state, selected: null };
  if (state.folder.length) return { ...state, folder: state.folder.slice(0, -1) };
  return state;
}

export function restore(page, state) {
  if (folderExists(page, state.folder)) return { state, notice: null };
  return {
    state: { ...state, folder: [], selected: null },
    notice: `${state.folder.join('/')}/ no longer exists in this scan; showing the top level.`,
  };
}

export function zoomAt([x, y, w, h], factor, fx, fy) {
  return [fx - (fx - x) / factor, fy - (fy - y) / factor, w / factor, h / factor];
}

export function panBy([x, y, w, h], dx, dy) {
  return [x + dx, y + dy, w, h];
}

// The viewBox that shows the whole diagram. A diagram bigger than the stage is
// left alone and the SVG shrinks it to fit; a smaller one is grown to the
// stage's size, so 1 unit is 1px and text never renders larger than its font
// size. The growth keeps the diagram centred across and at the top. `covered`
// is the width an open card hides at the right: the diagram is then fitted and
// centred in the width left over, so none of it lies under the card.
export function fitted([x, y, w, h], stageW, stageH, covered = 0) {
  const room = covered < stageW ? stageW - covered : stageW;
  if (w <= room && h <= stageH) return [x - (room - w) / 2, y, stageW, stageH];
  if (room === stageW) return [x, y, w, h];
  const s = Math.min(room / w, stageH / h);
  return [x - (room - w * s) / (2 * s), y, stageW / s, stageH / s];
}

export function start(doc, win) {
  const page = JSON.parse(doc.getElementById('data').textContent);
  const header = doc.getElementById('header');
  const diagram = doc.getElementById('diagram');
  const card = doc.getElementById('card');
  let { state, notice } = restore(page, parseHash(win.location.hash));
  let view = null;
  let pos = null;
  let zoomed = null; // a viewBox while zoomed or panned; null means fitted

  const svg = () => diagram.querySelector('svg');
  let content = null; // the diagram's own viewBox, before it is fitted to the stage
  const fit = () => fitted(content, svg().clientWidth, svg().clientHeight, card.hidden ? 0 : card.offsetWidth);
  const current = () => zoomed || fit();
  const setView = (vb) => { zoomed = vb; svg().setAttribute('viewBox', vb.join(' ')); };

  async function draw() {
    view = viewAt(page, state.folder);
    const keys = [...view.boxes, ...view.libraries].map((n) => n.key);
    if (state.selected && !keys.includes(state.selected)) state = { ...state, selected: null };
    pos = await layout(view);
    zoomed = null; // fit to view on load and after every drill-down
    paint();
  }

  function paint() {
    header.innerHTML = renderHeader(page, view, notice, state);
    // The card first: a fitted view is centred in the width it leaves.
    card.hidden = !state.selected;
    card.innerHTML = state.selected ? renderCard(page, view, state.selected) : '';
    diagram.innerHTML = renderDiagram(view, pos, state);
    if (svg()) {
      content = svg().getAttribute('viewBox').split(' ').map(Number);
      svg().setAttribute('viewBox', current().join(' '));
    }
    win.history.replaceState(null, '', formatHash(state) || win.location.pathname);
  }

  const open = (folder) => { state = { ...state, folder, selected: null }; notice = null; return draw(); };
  const set = (changes) => { state = { ...state, ...changes }; paint(); };
  const openSelected = () => {
    const box = view.boxes.find((b) => b.key === state.selected);
    if (box && box.kind !== 'module') open(box.key.split('/'));
  };

  let drag = null;
  let dragged = false;
  // The first click of a double-click selects the box; the second is ignored,
  // and the double-click then opens whatever the first one selected.
  diagram.addEventListener('click', (e) => {
    if (dragged || e.detail > 1) return;
    const node = e.target.closest('[data-key]');
    set({ selected: node ? node.dataset.key : null });
  });
  diagram.addEventListener('dblclick', openSelected);
  header.addEventListener('click', (e) => {
    const crumb = e.target.closest('[data-folder]');
    if (crumb) open(crumb.dataset.folder ? crumb.dataset.folder.split('/') : []);
    else if (e.target.closest('[data-toggle-arrows]')) set({ arrows: !state.arrows });
  });
  card.addEventListener('click', (e) => {
    const button = e.target.closest('[data-open]');
    if (button) open(button.dataset.open.split('/'));
  });
  doc.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      const next = onEscape(state);
      if (next.folder.length !== state.folder.length) open(next.folder);
      else set(next);
    } else if (e.key === 'Enter') {
      openSelected();
    } else if (e.key === '0' && svg()) {
      zoomed = null;
      paint();
    }
  });
  diagram.addEventListener('wheel', (e) => {
    if (!svg()) return;
    e.preventDefault();
    const perPixel = current()[2] / svg().clientWidth;
    if (e.ctrlKey || e.metaKey) {
      const pt = new DOMPoint(e.clientX, e.clientY).matrixTransform(svg().getScreenCTM().inverse());
      setView(zoomAt(current(), Math.exp(-e.deltaY * 0.002), pt.x, pt.y));
    } else {
      setView(panBy(current(), e.deltaX * perPixel, e.deltaY * perPixel));
    }
  }, { passive: false });
  diagram.addEventListener('pointerdown', (e) => {
    if (svg()) drag = { x: e.clientX, y: e.clientY, vb: current() };
    dragged = false;
  });
  win.addEventListener('pointermove', (e) => {
    if (!drag) return;
    const dx = e.clientX - drag.x;
    const dy = e.clientY - drag.y;
    if (Math.abs(dx) + Math.abs(dy) > 3) dragged = true;
    if (dragged) {
      const perPixel = drag.vb[2] / svg().clientWidth;
      setView(panBy(drag.vb, -dx * perPixel, -dy * perPixel));
    }
  });
  win.addEventListener('pointerup', () => { drag = null; });
  win.addEventListener('resize', () => { if (svg() && !zoomed) svg().setAttribute('viewBox', fit().join(' ')); });
  win.addEventListener('hashchange', () => {
    ({ state, notice } = restore(page, parseHash(win.location.hash)));
    draw();
  });
  draw();
}
