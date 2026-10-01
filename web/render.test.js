import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { viewAt } from './model.js';
import { esc, badgeText, renderDiagram, renderHeader, renderCard } from './render.js';

const page = JSON.parse(readFileSync(new URL('./testdata/shop.page.json', import.meta.url)));

// Positions without ELK: boxes in a row, each arrow a straight line.
function fakePos(view) {
  const nodes = {};
  let x = 0;
  for (const n of [...view.boxes, ...view.libraries]) { nodes[n.key] = { x, y: 0, w: n.width, h: n.height }; x += n.width + 20; }
  const edges = view.arrows.map((a) => ({ from: a.from, to: a.to, points: [[nodes[a.from].x, 40], [nodes[a.to].x, 0]] }));
  return { width: x, height: 60, nodes, edges };
}
const top = viewAt(page, []);
const on = { selected: null, arrows: true };

test('esc makes markup inert', () => {
  assert.equal(esc(`<a href="x">&'`), '&lt;a href=&quot;x&quot;&gt;&amp;&#39;');
});

test('badges show on-screen partners, then outside ones, and hide zero', () => {
  assert.equal(badgeText(3, 2), '3+2');
  assert.equal(badgeText(3, 0), '3');
  assert.equal(badgeText(0, 2), '+2');
  assert.equal(badgeText(0, 0), '');
});

test('the used-by badge rides 9 above its box, clear of the arrowheads; the uses badge 14 below', () => {
  const svg = renderDiagram(top, fakePos(top), on);
  const group = (key) => svg.match(new RegExp(`<g class="box[^"]*" data-key="${key}".*?</g>`))[0];
  const checkout = group('checkout'); // used by 1, uses 3, 38 tall
  assert.ok(/<text class="badge" x="[\d.]+" y="-9" text-anchor="end"><title>used by: shop<\/title>1<\/text>/.test(checkout));
  assert.ok(/<text class="badge" x="[\d.]+" y="52" text-anchor="end"><title>uses: [^<]*<\/title>3<\/text>/.test(checkout));
  assert.ok(!group('shop').includes('used by:')); // no badge when there is no partner
});

test('the diagram has a box per key, a lamp per box and an arrow per pair', () => {
  const svg = renderDiagram(top, fakePos(top), on);
  for (const b of top.boxes) assert.ok(svg.includes(`data-key="${b.key}"`));
  assert.ok(svg.includes('class="lamp lamp-red"'));
  assert.equal((svg.match(/class="arrow"/g) || []).length, top.arrows.length);
  assert.ok(svg.includes('checkout → pricing'));
});

test('selecting a box highlights its arrows and fades and dims the rest', () => {
  const svg = renderDiagram(top, fakePos(top), { selected: 'pricing', arrows: true });
  assert.equal((svg.match(/class="arrow hi"/g) || []).length, 2); // checkout→pricing, pricing→cart
  assert.equal((svg.match(/class="arrow faded"/g) || []).length, top.arrows.length - 2);
  assert.ok(/class="box selected" data-key="pricing"/.test(svg));
  assert.ok(/class="box dim" data-key="shop"/.test(svg));
  // highlighted arrows get the cream head, the rest keep the grey one
  assert.equal((svg.match(/class="arrow hi"[^>]*marker-end="url\(#head-hi\)"/g) || []).length, 2);
  assert.equal((svg.match(/class="arrow faded"[^>]*marker-end="url\(#head\)"/g) || []).length, top.arrows.length - 2);
});

test('with arrows off only the selected box keeps its arrows', () => {
  const svg = renderDiagram(top, fakePos(top), { selected: 'pricing', arrows: false });
  assert.equal((svg.match(/class="arrow/g) || []).length, 2);
  assert.equal((renderDiagram(top, fakePos(top), { selected: null, arrows: false }).match(/class="arrow/g) || []).length, 0);
});

test('names are escaped in the diagram and the card', () => {
  const evil = structuredClone(page);
  evil.modules.find((m) => m.id === 'src/pricing.ts').tree = ['<img src=x onerror=alert(1)>'];
  const view = viewAt(evil, []);
  const svg = renderDiagram(view, fakePos(view), on);
  assert.ok(!svg.includes('<img') && svg.includes('&lt;img'));
  assert.ok(!renderCard(evil, view, view.boxes.find((b) => b.name.startsWith('<')).key).includes('<img'));
});

test('an empty folder says so', () => {
  const empty = viewAt({ libraries: [], modules: [] }, []);
  assert.ok(renderDiagram(empty, { width: 0, height: 0, nodes: {}, edges: [] }, on).includes('Nothing to show'));
});

test('the header has a breadcrumb, a legend with counts, and when it was measured', () => {
  const html = renderHeader(page, viewAt(page, ['cart']), null, on);
  assert.ok(html.includes('data-folder=""') && html.includes('data-folder="cart"'));
  assert.ok(html.includes('1 medium') && html.includes('1 not measured'));
  assert.ok(html.includes('<span class="label">coverage</span> 2026-09-30 19:59'));
});

test('before any coverage the legend says how to get it, and a notice shows', () => {
  const html = renderHeader({ ...page, coverageAt: '' }, top, 'Folder gone/ was removed.', on);
  assert.ok(html.includes('umlv --metrics'));
  assert.ok(html.includes('class="notice"'));
});

test('a module card spells out its grade and lists functions worst first', () => {
  const html = renderCard(page, top, 'checkout');
  assert.ok(html.includes('Grade: red (μ + σ = 30.0; red is over 12)'));
  assert.ok(html.indexOf('>run<') < html.indexOf('>neverCalled<'));
  assert.ok(html.includes('class="worst"'));
  assert.ok(html.includes('href="vscode://file/repo/src/checkout.ts:9"'));
});

test('a folder card counts what was measured and offers Open', () => {
  const html = renderCard(page, top, 'cart');
  assert.ok(html.includes('lamp-dot lamp-amber'));
  assert.ok(html.includes('1 of 2 measured'));
  assert.ok(html.includes('data-open="cart"'));
  assert.ok(html.includes('basket'));
});

test('source links encode spaces', () => {
  const spaced = structuredClone(page);
  spaced.modules.find((m) => m.id === 'src/pricing.ts').source = 'src/my file.ts';
  assert.ok(renderCard(spaced, viewAt(spaced, []), 'pricing').includes('src/my%20file.ts'));
});
