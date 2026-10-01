export { run } from './checkout.js';
import { run } from './checkout.js';

// How a Cloudflare Worker exports its handler.
export default {
  fetch(): number {
    return run([]);
  },
} satisfies { fetch(): number };
