import type { Config } from '@react-router/dev/config';
import { glob } from 'node:fs/promises';
import { createGetUrl, getSlugs } from 'fumadocs-core/source';
import { getPageContentPath, getPageImagePath } from './app/lib/shared';

const getUrl = createGetUrl('/docs');

export default {
  ssr: true,
  async prerender({ getStaticPaths }) {
    const paths: string[] = [];
    const excluded: string[] = ['/api/search'];

    for (const path of getStaticPaths()) {
      if (!excluded.includes(path)) paths.push(path);
    }

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
