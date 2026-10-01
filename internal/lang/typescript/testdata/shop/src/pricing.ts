import type { Basket } from './cart/basket';

export function total(basket: Basket): number {
  return basket.size;
}

export const discounts = {
  half(price: number): number {
    return price / 2;
  },
  none: (price: number): number => price,
};
