import { source } from '@/lib/source';
import { canonicalUrl } from '@/lib/shared';

// Every page a search engine should know about, prerendered to a file like the
// rest of the site. The docs come from the same source the routes render, so a
// new page is listed without anyone remembering to add it. No <lastmod>: the
// only honest source for it is git history, which a shallow CI checkout lacks,
// and search engines ignore a date they learn not to trust.
//
// Each entry is the page's canonical URL, the one its own head names: a
// sitemap that disagrees with the page leaves a search engine to pick.
export function loader() {
  const urls = ['/', '/privacy', ...source.getPages().map((page) => page.url)]
    .map((path) => `  <url><loc>${canonicalUrl(path)}</loc></url>`)
    .join('\n');

  return new Response(
    `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${urls}
</urlset>
`,
    { headers: { 'Content-Type': 'application/xml; charset=utf-8' } },
  );
}
