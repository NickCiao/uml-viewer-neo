import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { viewAt, shorten, folderExists, GEOMETRY } from './model.js';

const page = JSON.parse(readFileSync(new URL('./testdata/shop.page.json', import.meta.url)));
const box = (view, key) => view.boxes.find((b) => b.key === key);

test('the top level shows one box per top-level name, plus used libraries', () => {
  const view = viewAt(page, []);
  assert.deepEqual(view.boxes.map((b) => b.key), ['cart', 'checkout', 'pricing', 'shop']);
  assert.deepEqual(view.libraries.map((l) => l.key), ['lib:zod']);
});

test("a folder that is also a module is one box with its folder's pooled lamp", () => {
  const cart = box(viewAt(page, []), 'cart');
  assert.equal(cart.kind, 'both');
  assert.equal(cart.moduleId, 'src/cart/index.ts');
  assert.equal(cart.grade, 'amber');
  assert.deepEqual(cart.pool, page.folders.cart);
  assert.deepEqual(cart.worst, { label: 'basket', grade: 'amber', score: 9 });
  assert.equal(cart.label, 'cart/ ›');
  assert.equal(cart.detail, '2 modules · 1 measured');
  assert.equal(cart.stats, null);
});

// A pooled folder can be green with a red module inside: the box names the red
// module count so the hotspot is not hidden; a red folder needs no such hint.
const pooled = (folderGrade) => ({ ...page, libraries: [], folders: {
  orders: { grade: folderGrade, stats: { mu: 4, sigma: 3.6, max: 13 }, functions: 4 } }, modules: [
  { id: 'o/cart', tree: ['orders', 'cart'], name: 'cart', source: '', grade: 'green', stats: { mu: 2, sigma: 1, max: 3 }, functions: [], uses: [] },
  { id: 'o/pay', tree: ['orders', 'pay'], name: 'pay', source: '', grade: 'red', stats: { mu: 13, sigma: 0, max: 13 }, functions: [], uses: [] },
  { id: 'o/x', tree: ['orders', 'x', 'y'], name: 'y', source: '', grade: 'unlit', functions: [], uses: [] },
] });

test('a folder takes its grade from the page data, not its worst module', () => {
  const orders = box(viewAt(pooled('green'), []), 'orders');
  assert.equal(orders.grade, 'green');
  assert.deepEqual(orders.worst, { label: 'pay', grade: 'red', score: 13 });
  assert.equal(orders.detail, '3 modules · 2 measured · 1 red');
  assert.equal(box(viewAt(pooled('red'), []), 'orders').detail, '3 modules · 2 measured');
});

test('a folder with no data is unlit and names no worst module', () => {
  const orders = box(viewAt({ ...pooled('green'), folders: {} }, ['orders']), 'orders/x');
  assert.equal(orders.grade, 'unlit');
  assert.equal(orders.worst, null);
  assert.equal(orders.pool, null);
});

test('a folder holding one module says so in the singular', () => {
  const one = { ...page, libraries: [], modules: [
    { id: 'cmd/app', tree: ['cmd', 'app'], name: 'app', source: 'cmd/app', grade: 'green', functions: [], uses: [] },
  ] };
  assert.equal(box(viewAt(one, []), 'cmd').detail, '1 module · 1 measured');
});

test('a plain module box carries its stats', () => {
  const checkout = box(viewAt(page, []), 'checkout');
  assert.equal(checkout.kind, 'module');
  assert.equal(checkout.grade, 'red');
  assert.equal(checkout.stats.mu, 16);
});

test('arrows are merged to one per pair of boxes and skip a box importing itself', () => {
  const arrows = viewAt(page, []).arrows.map((a) => `${a.from}>${a.to}`).sort();
  assert.deepEqual(arrows, ['checkout>cart', 'checkout>lib:zod', 'checkout>pricing', 'pricing>cart', 'shop>checkout']);
});

test('inside a folder, its own module is a box and outside partners become badge counts', () => {
  const view = viewAt(page, ['cart']);
  assert.deepEqual(view.boxes.map((b) => b.key), ['cart', 'cart/basket']);
  assert.equal(box(view, 'cart').kind, 'module');
  const basket = box(view, 'cart/basket');
  assert.deepEqual(basket.usedBy, { on: ['cart'], off: ['checkout', 'pricing'] });
  assert.deepEqual(basket.uses, { on: [], off: [] });
  assert.deepEqual(view.libraries, []);
});

test('the legend counts boxes at this level by lamp', () => {
  assert.deepEqual(viewAt(page, []).counts, { red: 1, amber: 1, green: 1, unlit: 1 });
});

test('an import cycle gives an arrow each way', () => {
  const cycle = { libraries: [], modules: [
    { id: 'a', tree: ['a'], grade: 'unlit', uses: ['b'], functions: [] },
    { id: 'b', tree: ['b'], grade: 'unlit', uses: ['a'], functions: [] }] };
  assert.deepEqual(viewAt(cycle, []).arrows.map((x) => x.from + x.to).sort(), ['ab', 'ba']);
});

test('shorten keeps both ends of a long name', () => {
  const s = shorten('abcdefghijklmnopqrstuvwxyz0123');
  assert.equal(s.length, 24);
  assert.ok(s.startsWith('abcdefghijkl') && s.endsWith('z0123') && s.includes('…'));
  assert.equal(shorten('short'), 'short');
});

test('folderExists only for folders with modules below them', () => {
  assert.equal(folderExists(page, []), true);
  assert.equal(folderExists(page, ['cart']), true);
  assert.equal(folderExists(page, ['pricing']), false);
  assert.equal(folderExists(page, ['gone']), false);
});

test('boxes are sized to fit their text', () => {
  const cart = box(viewAt(page, []), 'cart');
  assert.ok(cart.width >= 'cart/ ›'.length * GEOMETRY.nameW + 2 * GEOMETRY.padX);
  assert.equal(cart.height, 2 * GEOMETRY.line + 2 * GEOMETRY.padY);
  assert.equal(box(viewAt(page, []), 'shop').height, GEOMETRY.line + 2 * GEOMETRY.padY);
});
