// Assembles what GitHub Pages uploads, out of what `react-router build` leaves
// in build/client.
//
// Pages serves a project site under /<repo>, so the artifact's root *is*
// /solitary. react-router builds for that with a basename, which puts every
// prerendered page under build/client/solitary/ — one level too deep once the
// artifact is served at that prefix — while the assets it copies stay at the
// top. So the pages move up and the assets stay put.
//
// The script tags react-router writes into that HTML are the one thing
// `renderBuiltUrl` in vite.config.ts does not reach: they come from its own
// build manifest, which follows Vite's `base`, and `base` cannot be used here
// (see the comment there). They are rewritten below instead.

import { cp, readFile, readdir, rename, rm, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { basename } from '../app/lib/shared.ts';

const built = 'build/client';
const out = 'build/pages';
const prefix = basename.replace(/^\//, '');

async function* htmlFiles(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) yield* htmlFiles(path);
    else if (entry.name.endsWith('.html')) yield path;
  }
}

await rm(out, { recursive: true, force: true });

// The prerendered pages, lifted out of the basename directory...
await cp(join(built, prefix), out, { recursive: true });

// ...and everything else react-router left beside it: assets, fonts, favicon.
const beside = (await readdir(built, { withFileTypes: true })).filter(
  (entry) => entry.name !== prefix,
);
await Promise.all(
  beside.map((entry) => cp(join(built, entry.name), join(out, entry.name), { recursive: true })),
);

// Pages wants its fallback as one file at the root, not as the directory a
// prerendered route leaves behind.
await rename(join(out, '404', 'index.html'), join(out, '404.html'));
await rm(join(out, '404'), { recursive: true });

const pages = [];
for await (const file of htmlFiles(out)) pages.push(file);

const rewrites = await Promise.all(
  pages.map(async (file) => {
    const html = await readFile(file, 'utf8');
    const fixed = html.replaceAll('"/assets/', `"${basename}/assets/`);
    if (fixed === html) return 0;

    await writeFile(file, fixed);
    return 1;
  }),
);
const rewritten = rewrites.reduce((total, one) => total + one, 0);

// A page that kept the wrong asset URLs would still render, just unstyled and
// without hydrating, so say what happened rather than leaving it to be noticed.
console.log(`build/pages: ${rewritten} pages rewritten to serve from ${basename}`);
