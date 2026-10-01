import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { parseHash, formatHash, onEscape, restore, zoomAt, panBy } from './app.js';

const page = JSON.parse(readFileSync(new URL('./testdata/shop.page.json', import.meta.url)));

test('the URL fragment round-trips the folder, the selection and the arrows switch', () => {
  const state = { folder: ['cart'], selected: 'cart/basket', arrows: false };
  assert.deepEqual(parseHash(formatHash(state)), state);
  assert.deepEqual(parseHash(''), { folder: [], selected: null, arrows: true });
  assert.deepEqual(parseHash('#f=cart&s=cart/basket'), { folder: ['cart'], selected: 'cart/basket', arrows: true });
  assert.equal(formatHash({ folder: [], selected: null, arrows: true }), '');
});

test('Esc closes the card first, then goes up a level, then does nothing', () => {
  const s = { folder: ['cart'], selected: 'cart/basket', arrows: true };
  const once = onEscape(s);
  assert.deepEqual(once, { ...s, selected: null });
  assert.deepEqual(onEscape(once).folder, []);
  const top = { folder: [], selected: null, arrows: true };
  assert.deepEqual(onEscape(top), top);
});

test('a link into a folder that a re-scan removed opens the top with a notice', () => {
  const { state, notice } = restore(page, { folder: ['gone'], selected: 'gone/x', arrows: false });
  assert.deepEqual(state, { folder: [], selected: null, arrows: false });
  assert.ok(notice.includes('gone/'));
  assert.equal(restore(page, { folder: ['cart'], selected: null, arrows: true }).notice, null);
});

test('zooming keeps the point under the pointer still, and panning moves the view', () => {
  assert.deepEqual(zoomAt([0, 0, 100, 100], 2, 50, 50), [25, 25, 50, 50]);
  assert.deepEqual(zoomAt([0, 0, 100, 100], 2, 0, 0), [0, 0, 50, 50]);
  assert.deepEqual(panBy([0, 0, 100, 100], 10, -5), [10, -5, 100, 100]);
});
