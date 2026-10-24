// Turns what `react-router build` leaves in build/client into what GitHub Pages
// uploads. The site is served from the root of solitary.balakin.io, so the
// build output is already the site — everything that used to move, rewrite or
// re-prefix a URL here went away with the /solitary path prefix.
//
// What is left is where a page's HTML lives. Prerendering writes every route
// as a directory with an index.html in it, and Pages answers the bare path of a
// directory with a redirect to the slashed one: /docs/quickstart would 301 to
// /docs/quickstart/, while every link fumadocs builds, and the canonical URL,
// is the bare path. Pages serves foo.html at /foo without a redirect, so each
// page becomes that file instead. A directory that still holds other pages
// keeps its index.html: a file and a directory of the same name is a pair
// whose resolution Pages does not document, so the build never makes one.
//
// The catch-all route becomes 404.html by the same rule: Pages answers an
// unknown path with that file from the root of what it serves.

import { glob, readdir, rename, rmdir } from 'node:fs/promises';
import { dirname, join } from 'node:path';

const out = 'build/client';
const moved = [];

for await (const page of glob('**/index.html', { cwd: out })) {
  const dir = dirname(page);
  if (dir === '.') continue;
  if ((await readdir(join(out, dir))).length > 1) continue;

  await rename(join(out, page), join(out, `${dir}.html`));
  await rmdir(join(out, dir));
  moved.push(dir);
}

console.log(`${out}: ${moved.length} pages are now files of their own, 404.html among them`);
