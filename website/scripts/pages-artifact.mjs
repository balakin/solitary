// Turns what `react-router build` leaves in build/client into what GitHub Pages
// uploads. The site is served from the root of solitary.balakin.io, so the
// build output is already the site — everything that used to move, rewrite or
// re-prefix a URL here went away with the /solitary path prefix.
//
// What is left is the one thing prerendering cannot express: Pages answers an
// unknown path with 404.html from the root of what it serves, and a prerendered
// route is a directory with an index.html in it.

import { rename, rm } from 'node:fs/promises';
import { join } from 'node:path';

const out = 'build/client';

await rename(join(out, '404', 'index.html'), join(out, '404.html'));
await rm(join(out, '404'), { recursive: true });

console.log(`${out}: /404 is now 404.html`);
