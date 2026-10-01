// Checkout: three paths in run — the function, the loop, the condition.
import { readFileSync } from 'node:fs';
import type { ZodType } from 'zod';

import { Basket } from './cart/basket.js';
import * as cart from './cart';
import { total } from '@/pricing';

export function run(items: string[]): number {
  const basket = new Basket();
  for (const item of items) {
    if (item) {
      basket.add(item);
    }
  }
  return total(basket);
}

export const neverCalled = (schema: ZodType) => String(schema) + String(readFileSync) + String(cart);
