import { expect, test } from 'vitest';
import { Basket } from './cart/basket';
import { run } from './checkout';

test('run', () => {
  run(['a', '']);
});

test('full', () => {
  const b = new Basket();
  b.add('a');
  b.add('b');
  expect(b.full()).toBe(false);
  b.add('c');
  expect(b.full()).toBe(true);
});
