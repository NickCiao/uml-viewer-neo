import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { viewAt, worstGrade, shorten, folderExists, GEOMETRY } from './model.js';

const page = JSON.parse(readFileSync(new URL('./testdata/shop.page.json', import.meta.url)));
const box = (view, key) => view.boxes.find((b) => b.key === key);

test('the top level shows one box per top-level name, plus used libraries', () => {
  const view = viewAt(page, []);
  assert.deepEqual(view.boxes.map((b) => b.key), ['cart', 'checkout', 'pricing', 'shop']);
  assert.deepEqual(view.libraries.map((l) => l.key), ['lib:zod']);
});

test('a folder that is also a module is one box with its worst measured lamp', () => {
  const cart = box(viewAt(page, []), 'cart');
  assert.equal(cart.kind, 'both');
  assert.equal(cart.moduleId, 'src/cart/index.ts');
  assert.equal(cart.grade, 'amber');
  assert.equal(cart.label, 'cart/ ›');
  assert.equal(cart.detail, '2 modules · 1 measured');
  assert.equal(cart.stats, null);
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

test('worstGrade ignores unlit unless nothing is lit', () => {
  assert.equal(worstGrade(['unlit', 'green']), 'green');
  assert.equal(worstGrade(['green', 'red', 'amber']), 'red');
  assert.equal(worstGrade(['unlit']), 'unlit');
  assert.equal(worstGrade([]), 'unlit');
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
