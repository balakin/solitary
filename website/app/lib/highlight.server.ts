import type { Root } from 'hast';
import { highlightHast } from 'fumadocs-core/highlight';

/**
 * Highlights code into a tree a loader can return, for HighlightedCode to
 * render. A loader runs at build time, where every page is prerendered, so the
 * page arrives in colour and Shiki never ships to the browser. fumadocs'
 * DynamicCodeBlock highlights in an effect instead, which leaves the code plain
 * until the page has hydrated.
 */
export function highlightCode(code: string, lang: string): Promise<Root> {
  return highlightHast(code, {
    lang,
    themes: { light: 'github-light', dark: 'github-dark' },
    defaultColor: false,
  });
}
