import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { parseHash, formatHash, onEscape, restore, zoomAt, panBy, fitted } from './app.js';

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

test('fitting shrinks a big diagram to the stage but never enlarges a small one', () => {
  // bigger than the stage in either dimension: the viewBox is left to shrink it
  assert.deepEqual(fitted([-24, -24, 3000, 800], 1600, 958), [-24, -24, 3000, 800]);
  assert.deepEqual(fitted([-24, -24, 500, 2000], 1600, 958), [-24, -24, 500, 2000]);
  // smaller in both: grown to the stage's pixels (1 unit = 1px), centred across, top-aligned
  assert.deepEqual(fitted([-24, -24, 500, 300], 1600, 958), [-574, -24, 1600, 958]);
  // exactly the stage's size is already 1:1
  assert.deepEqual(fitted([-24, -24, 1600, 958], 1600, 958), [-24, -24, 1600, 958]);
});

// Where the diagram's box lands on the stage, in px, when the SVG draws vb
// across the whole stage (vb has the stage's shape, so nothing is letterboxed).
function landing(vb, [x, y, w, h], stageW) {
  const s = stageW / vb[2];
  return { left: (x - vb[0]) * s, right: (x + w - vb[0]) * s, top: (y - vb[1]) * s, bottom: (y + h - vb[1]) * s };
}
const near = (actual, expected) => assert.ok(Math.abs(actual - expected) < 1e-6, `${actual} is not ${expected}`);

test('a covered stage centres a small diagram in the width the card leaves', () => {
  // 1600 - 380 leaves 1220: centred there, still 1:1 and top-aligned
  assert.deepEqual(fitted([-24, -24, 500, 300], 1600, 958, 380), [-384, -24, 1600, 958]);
});

test('a covered stage shrinks a diagram that no longer fits, to the width the card leaves', () => {
  const wide = [-24, -24, 3000, 800];
  const vb = fitted(wide, 1600, 958, 380);
  near(vb[2] / vb[3], 1600 / 958); // the stage's shape, so the SVG adds no margin of its own
  const at = landing(vb, wide, 1600);
  near(at.left, 0);
  near(at.right, 1220); // not a pixel under the card
  near(at.top, 0);

  const tall = [-24, -24, 800, 2000]; // limited by height: centred in the uncovered part
  const t = landing(fitted(tall, 1600, 958, 380), tall, 1600);
  near(t.left + t.right, 1220);
  near(t.top, 0);
  near(t.bottom, 958);
});

test('without a card, fitting is what it always was', () => {
  assert.deepEqual(fitted([-24, -24, 3000, 800], 1600, 958, 0), [-24, -24, 3000, 800]);
  assert.deepEqual(fitted([-24, -24, 500, 300], 1600, 958, 0), [-574, -24, 1600, 958]);
  assert.deepEqual(fitted([-24, -24, 1600, 958], 1600, 958, 0), [-24, -24, 1600, 958]);
});

test('a card as wide as the stage is ignored, not divided by', () => {
  assert.deepEqual(fitted([-24, -24, 500, 300], 300, 958, 380), fitted([-24, -24, 500, 300], 300, 958));
});
