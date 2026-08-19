import type { Config } from '@react-router/dev/config';
import { glob } from 'node:fs/promises';
import { createGetUrl, getSlugs } from 'fumadocs-core/source';
import { basename, getPageContentPath, getPageImagePath } from './app/lib/shared.ts';

const getUrl = createGetUrl('/docs');

export default {
  ssr: true,
  // GitHub Pages serves this repository's site under /solitary, so every route
  // and every asset URL is one path segment deeper than it is in development.
  basename,
  // Every route is in the manifest the first document ships with. The default
  // under `ssr: true` is to discover routes lazily, which makes the client ask
  // /__manifest for the routes a link points at before navigating to it — a
  // server endpoint, so on a static host every client-side navigation fails.
  routeDiscovery: { mode: 'initial' },
  async prerender({ getStaticPaths }) {
    // Everything, /api/search included: it is the search index rather than a
    // route, and on a static host it has to be a file on disk.
    // GitHub Pages answers an unknown path with 404.html from the root of what
    // it serves, so the catch-all route is prerendered once to become that file.
    const paths: string[] = [...getStaticPaths(), '/404'];

    for await (const entry of glob('**/*.mdx', { cwd: 'content/docs' })) {
      const slugs = getSlugs(entry);

      paths.push(getUrl(slugs));
      paths.push(getPageImagePath(slugs));
      // The page as plain Markdown. Prerendered alongside the HTML so that
      // /docs/<page>.md is a file on disk rather than a route only a running
      // server can answer.
      paths.push(getPageContentPath(slugs));
    }

    return paths;
  },
} satisfies Config;
