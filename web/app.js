// app.js: wiring between the page's DOM and the model, layout and render.
import { viewAt } from './model.js';
import { layout } from './layout.js';
import { renderDiagram, renderHeader } from './render.js';

export function start(doc) {
  const page = JSON.parse(doc.getElementById('data').textContent);
  const ui = { folder: [], selected: null, arrows: true };
  const draw = async () => {
    const view = viewAt(page, ui.folder);
    const pos = await layout(view);
    doc.getElementById('header').innerHTML = renderHeader(page, view, null, ui);
    doc.getElementById('diagram').innerHTML = renderDiagram(view, pos, ui);
  };
  doc.addEventListener('dblclick', (e) => {
    const box = e.target.closest('.box.folder');
    if (box) { ui.folder = box.dataset.key.split('/'); draw(); }
  });
  doc.addEventListener('click', (e) => {
    const crumb = e.target.closest('[data-folder]');
    if (crumb) { ui.folder = crumb.dataset.folder ? crumb.dataset.folder.split('/') : []; draw(); }
  });
  draw();
}
