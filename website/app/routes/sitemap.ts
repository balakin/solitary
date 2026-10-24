import { source } from '@/lib/source';
import { siteUrl } from '@/lib/shared';

// Every page a search engine should know about, prerendered to a file like the
// rest of the site. The docs come from the same source the routes render, so a
// new page is listed without anyone remembering to add it. No <lastmod>: the
// only honest source for it is git history, which a shallow CI checkout lacks,
// and search engines ignore a date they learn not to trust.
//
// Each URL ends in a slash because a prerendered page is a directory holding
// an index.html, and GitHub Pages answers the bare path with a redirect to the
// slashed one; a sitemap entry that redirects is one a search engine reports
// as an error rather than indexes.
export function loader() {
  const paths = ['/privacy', ...source.getPages().map((page) => page.url)];
  const urls = ['/', ...paths.map((path) => `${path}/`)]
    .map((path) => `  <url><loc>${siteUrl}${path}</loc></url>`)
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
