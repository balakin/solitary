import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach, vi } from 'vitest';

// jsdom has no layout and so no media queries, and fumadocs' theme provider
// asks for one on mount. Nothing tested depends on the answer: every query
// reports that it does not match, and that it never will.
window.matchMedia = vi.fn((query: string) => ({
  matches: false,
  media: query,
  onchange: null,
  addEventListener: () => {},
  removeEventListener: () => {},
  addListener: () => {},
  removeListener: () => {},
  dispatchEvent: () => false,
}));

// Testing Library unmounts after each test on its own only when the runner
// exposes globals, which this config does not.
afterEach(() => {
  cleanup();
  delete window.umami;
});
