import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { viewAt } from './model.js';
import { layout } from './layout.js';

const page = JSON.parse(readFileSync(new URL('./testdata/shop.page.json', import.meta.url)));
const overlap = (a, b) => a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h;

test('every box and library gets a place and none overlap', async () => {
  const view = viewAt(page, []);
  const pos = await layout(view);
  const keys = [...view.boxes, ...view.libraries].map((n) => n.key);
  assert.deepEqual(Object.keys(pos.nodes).sort(), keys.sort());
  for (const a of keys) for (const b of keys) if (a < b) assert.ok(!overlap(pos.nodes[a], pos.nodes[b]), `${a} overlaps ${b}`);
  assert.ok(pos.width > 0 && pos.height > 0);
});

test('importers sit above what they import, and every arrow has a route', async () => {
  const view = viewAt(page, []);
  const pos = await layout(view);
  for (const e of pos.edges) {
    assert.ok(pos.nodes[e.from].y + pos.nodes[e.from].h <= pos.nodes[e.to].y, `${e.from} is not above ${e.to}`);
    assert.ok(e.points.length >= 2);
  }
});

test('an import cycle still lays out', async () => {
  const cycle = { libraries: [], modules: [
    { id: 'a', tree: ['a'], grade: 'unlit', uses: ['b'], functions: [] },
    { id: 'b', tree: ['b'], grade: 'unlit', uses: ['a'], functions: [] }] };
  const pos = await layout(viewAt(cycle, []));
  assert.equal(pos.edges.length, 2);
});

test('a crowded folder of 60 boxes and about 150 arrows lays out within 3 seconds', async () => {
  const modules = Array.from({ length: 60 }, (_, i) => ({
    id: `m${i}`, tree: [`module${String(i).padStart(2, '0')}`], grade: 'unlit', functions: [],
    uses: [1, 7, 13].map((k) => `m${(i * k + 5) % 60}`).filter((u) => u !== `m${i}`),
  }));
  const started = performance.now();
  const pos = await layout(viewAt({ libraries: [], modules }, []));
  assert.equal(Object.keys(pos.nodes).length, 60);
  assert.ok(performance.now() - started < 3000, `took ${Math.round(performance.now() - started)} ms`);
});

test('an empty folder lays out to nothing', async () => {
  const pos = await layout(viewAt({ libraries: [], modules: [] }, []));
  assert.deepEqual(pos.nodes, {});
});
