import { loader } from 'fumadocs-core/source';
import { pageSchema } from 'fumadocs-core/source/schema';
import { defineDocs } from 'fumadocs-mdx/macro';
import { z } from 'zod';
import { docsRoute, getPageContentPath } from './shared';

export const docs = defineDocs({
  dir: 'content/docs',
  docs: {
    schema: pageSchema.extend({
      // What a search result says about the page. `description` is shown under
      // the page's title as well, where a line is right; a result wants the
      // 120 to 160 characters a search engine shows before it cuts, and a
      // shorter one is often replaced by an excerpt of the engine's choosing.
      seoDescription: z.string().min(120).max(160),
    }),
    async: true,
    postprocess: {
      includeProcessedMarkdown: true,
    },
  },
});

export const source = loader({
  source: docs.toFumadocsSource(),
  baseUrl: docsRoute,
});

export function getPageMarkdownUrl(page: (typeof source)['$inferPage']) {
  return {
    segments: [...page.slugs, 'content.md'],
    url: getPageContentPath(page.slugs, page.locale),
  };
}

export async function getLLMText(page: (typeof source)['$inferPage']) {
  const processed = await page.data.getText('processed');
  // The description is the page's own one-line summary. Without it, a reader
  // that has only this file has to infer from the body what the page is for.
  const heading = [`# ${page.data.title} (${page.url})`, page.data.description]
    .filter(Boolean)
    .join('\n\n');

  return `${heading}

${processed.trimStart()}`;
}
