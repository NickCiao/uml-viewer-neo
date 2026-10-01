export class Basket {
  private items: string[] = [];

  add(item: string): void {
    this.items.push(item.trim());
  }

  full = (): boolean => this.items.length >= 3;

  get size(): number {
    return this.items.length;
  }
}
