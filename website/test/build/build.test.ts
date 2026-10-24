// Checks the prerendered site in build/client — what GitHub Pages serves — rather
// than the components, so it fails on what a visitor would actually receive.
// What a component does once rendered is tested in test/components; what is
// left here only the build knows: which pages became files, what each route's
// meta put in its head, and what a build with or without a website ID ships.
// Run `pnpm build:pages` first; CI runs it in that order.

import { existsSync, readFileSync } from 'node:fs';
import { glob } from 'node:fs/promises';
import { join } from 'node:path';
import { beforeAll, expect, test } from 'vitest';

const out = 'build/client';
const site = 'https://solitary.balakin.io';
const repoUrl = 'https://github.com/balakin/solitary';
const licenseUrl = `${repoUrl}/blob/main/LICENSE`;

const read = (path: string) => readFileSync(join(out, path), 'utf8');

/** Every opening tag in `html` whose name is `tag` and which contains `needle`. */
function tags(html: string, tag: string, needle: string) {
  return [...html.matchAll(new RegExp(`<${tag}\\b[^>]*>`, 'g'))]
    .map(([match]) => match)
    .filter((match) => match.includes(needle));
}

/** Whether `html` is react-router's prerendered redirect rather than a page. */
const isRedirect = (html: string) => html.includes('<meta http-equiv="refresh"');

/**
 * Every page the build ships, by the address Pages serves it at: index.html at
 * the root, and every other page as <path>.html at <path> (see
 * scripts/pages-artifact.mjs). The 404 page and redirects are not pages.
 */
async function pages() {
  const found: { file: string; url: string }[] = [];
  for await (const file of glob('**/*.html', { cwd: out })) {
    if (file === '404.html' || isRedirect(read(file))) continue;
    const path = file === 'index.html' ? '' : file.replace(/(\/index)?\.html$/, '');
    found.push({ file, url: `${site}/${path}` });
  }
  return found;
}

/** Whether `html` has a link to `href` whose text is `text`. */
function hasLink(html: string, href: string, text: string) {
  return new RegExp(`<a\\b[^>]*href="${href}"[^>]*>(?:<!-- -->)?${text}<`).test(html);
}

beforeAll(() => {
  expect(
    existsSync(join(out, '404.html')),
    `${out} is missing or incomplete: run \`pnpm build:pages\` first`,
  ).toBe(true);
});

test('/privacy is prerendered, so it loads directly and on refresh', () => {
  expect(existsSync(join(out, 'privacy.html'))).toBe(true);
});

// Pages serves <path>.html at <path> and a directory's index.html at <path>/,
// redirecting <path> there. Which one wins when both exist is not documented,
// so the build never ships the pair: a page is a file, or a directory index
// when other pages live under it.
test('no page is both a file and a directory', async () => {
  for await (const file of glob('**/*.html', { cwd: out })) {
    const dir = join(out, file.slice(0, -'.html'.length));
    expect(existsSync(dir), `${file} sits next to a directory of the same name`).toBe(false);
  }
});

test('/docs sends the browser to the first page at once, and asks not to be indexed', () => {
  const html = read('docs/index.html');
  expect(html).toContain('<meta http-equiv="refresh" content="0;url=/docs/overview">');
  expect(html).toContain('<meta name="robots" content="noindex">');
  expect(existsSync(join(out, 'docs', 'overview.html'))).toBe(true);
});

// Pages used to serve every page at its slashed address, and those were indexed
// and shared. Nothing is there now, so Pages answers with 404.html, which has
// to send the browser on before the app starts.
test('the 404 page sends a slashed address on to the bare one', () => {
  expect(read('404.html')).toMatch(
    /<script>if \(location\.pathname\.length > 1 && location\.pathname\.endsWith\('\/'\)\) location\.replace\(/,
  );
});

// Search engines show about 160 characters of a description and cut the rest
// mid-phrase; the docs pages are held to the same by their schema.
test.each(['index.html', 'privacy.html'])('%s has a description a search result shows whole', (page) => {
  const description = read(page).match(/<meta name="description" content="([^"]*)"/)?.[1] ?? '';
  expect(description.length, description).toBeGreaterThanOrEqual(120);
  expect(description.length, description).toBeLessThanOrEqual(160);
});

test('the privacy policy has its own title and description', () => {
  const html = read('privacy.html');
  expect(html).toContain('<title>Privacy Policy | Solitary</title>');
  expect(html).toMatch(/<meta name="description" content="[^"]+"/);
});

// The footer's links are tested on the component; this is that every page
// that should render it does.
test.each(['index.html', 'privacy.html', 'docs/overview.html', '404.html'])(
  '%s ships the footer',
  (page) => {
    const html = read(page);
    expect(hasLink(html, repoUrl, 'GitHub'), 'GitHub').toBe(true);
    expect(hasLink(html, licenseUrl, 'Apache 2.0 License'), 'licence').toBe(true);
    expect(hasLink(html, '/privacy', 'Privacy Policy'), 'privacy policy').toBe(true);
  },
);

test('each page loads the tracker once, and only when the build had a website ID', async () => {
  const id = process.env.VITE_UMAMI_WEBSITE_ID?.trim();
  let pages = 0;

  for await (const page of glob('**/*.html', { cwd: out })) {
    const scripts = tags(read(page), 'script', 'cloud.umami.is');
    pages++;

    if (!id) {
      expect(scripts, `${page} loads a tracker with no website ID`).toHaveLength(0);
      continue;
    }
    expect(scripts, `${page} should load the tracker exactly once`).toHaveLength(1);
    expect(scripts[0]).toContain('src="https://cloud.umami.is/script.js"');
    expect(scripts[0]).toContain(`data-website-id="${id}"`);
    expect(scripts[0]).toContain('data-domains="solitary.balakin.io"');
    expect(scripts[0]).toContain('defer');
  }

  expect(pages, 'no prerendered pages found').toBeGreaterThan(1);
});

// Crawlers fetch the image by the URL in the tag, so it has to be absolute and
// has to name a file the build actually ships: either mistake leaves a link
// preview with no picture and no error anywhere.
test.each([
  ['index.html', `${site}/og.png`],
  ['privacy.html', `${site}/og.png`],
  ['docs/overview.html', `${site}/og/docs/overview/image.webp`],
])('%s names an absolute social image that the build ships', (page, image) => {
  const html = read(page);
  expect(tags(html, 'meta', `property="og:image" content="${image}"`)).toHaveLength(1);
  expect(tags(html, 'meta', `name="twitter:image" content="${image}"`)).toHaveLength(1);
  expect(existsSync(join(out, image.slice(site.length))), `${image} is not in the build`).toBe(
    true,
  );
});

// The sitemap is built from the docs source rather than from what was
// prerendered, so this is what catches the two drifting apart: a page the build
// ships and the sitemap leaves out, or an entry that would answer with a 404.
test('the sitemap lists exactly the pages the build ships', async () => {
  const listed = [...read('sitemap.xml').matchAll(/<loc>([^<]+)<\/loc>/g)].map(([, url]) => url);
  const shipped = (await pages()).map(({ url }) => url);

  expect(new Set(listed)).toEqual(new Set(shipped));
  expect(listed, 'the sitemap lists a page twice').toHaveLength(shipped.length);
});

test('robots.txt allows crawling and points at the sitemap', () => {
  const robots = read('robots.txt');
  expect(robots).not.toMatch(/^Disallow: \/\S/m);
  expect(robots).toMatch(/^Sitemap: https:\/\/solitary\.balakin\.io\/sitemap\.xml$/m);
});

// One address per page, named the same way in three places that a search
// engine compares: the page's canonical link, its og:url, and the sitemap.
// Any disagreement leaves the engine to choose.
test('every page names itself, once, as its canonical URL, and the sitemap agrees', async () => {
  const listed = new Set(
    [...read('sitemap.xml').matchAll(/<loc>([^<]+)<\/loc>/g)].map(([, url]) => url),
  );
  const shipped = await pages();

  for (const { file: page, url: self } of shipped) {
    const html = read(page);

    expect(tags(html, 'link', 'rel="canonical"'), page).toEqual([
      `<link rel="canonical" href="${self}"/>`,
    ]);
    expect(tags(html, 'meta', 'property="og:url"'), page).toEqual([
      `<meta property="og:url" content="${self}"/>`,
    ]);
    expect(listed.has(self), `${self} is not in the sitemap`).toBe(true);
  }

  expect(shipped.length, 'no prerendered pages found').toBeGreaterThan(1);
});

test('the 404 page claims no canonical URL', () => {
  const html = read('404.html');
  expect(tags(html, 'link', 'rel="canonical"')).toHaveLength(0);
  expect(tags(html, 'meta', 'property="og:url"')).toHaveLength(0);
});

// A bare "Quickstart" in a list of search results could be anyone's. The
// preview card keeps the bare name: it shows the site on a line of its own.
test('every docs page is titled with its own name and then the site', async () => {
  let checked = 0;

  for (const { file } of await pages()) {
    if (!file.startsWith('docs/')) continue;
    const html = read(file);
    const heading = html.match(/<h1[^>]*>([^<]+)<\/h1>/)?.[1];
    checked++;

    expect(heading, `${file} has no heading`).toBeTruthy();
    expect(html, file).toContain(`<title>${heading} | Solitary</title>`);
    expect(tags(html, 'meta', 'property="og:title"'), file).toEqual([
      `<meta property="og:title" content="${heading}"/>`,
    ]);
  }

  expect(checked, 'no docs pages found').toBeGreaterThan(1);
});

// The subtitle under a docs page's title is a line; a search result wants the
// 120 to 160 characters an engine shows before cutting, or it substitutes an
// excerpt of its own. Each page carries one of each (seoDescription).
test('every docs page tells search engines more than its subtitle does', async () => {
  let checked = 0;

  for (const { file } of await pages()) {
    if (!file.startsWith('docs/')) continue;
    const html = read(file);
    const description = html.match(/<meta name="description" content="([^"]*)"/)?.[1] ?? '';
    const decoded = description.replaceAll('&#x27;', "'").replaceAll('&amp;', '&');
    checked++;

    expect(decoded.length, `${file}: "${decoded}"`).toBeGreaterThanOrEqual(120);
    expect(decoded.length, `${file}: "${decoded}"`).toBeLessThanOrEqual(160);
    expect(tags(html, 'meta', 'property="og:description"'), file).toEqual([
      `<meta property="og:description" content="${description}"/>`,
    ]);
    expect(html, `${file} shows its search description as the subtitle`).not.toMatch(
      new RegExp(`<p[^>]*>${description.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}</p>`),
    );
  }

  expect(checked, 'no docs pages found').toBeGreaterThan(1);
});

// Parsed rather than matched: a stray character makes the whole block invalid,
// and a search engine then ignores it without saying so.
test('the home page describes the site and the software as structured data', () => {
  const scripts = [
    ...read('index.html').matchAll(/<script type="application\/ld\+json">([^<]*)<\/script>/g),
  ];
  expect(scripts).toHaveLength(1);

  const graph: Record<string, unknown>[] = JSON.parse(scripts[0][1])['@graph'];
  expect(graph.find((node) => node['@type'] === 'WebSite')).toMatchObject({
    name: 'Solitary',
    url: `${site}/`,
  });
  expect(graph.find((node) => node['@type'] === 'SoftwareApplication')).toMatchObject({
    name: 'Solitary',
    url: `${site}/`,
    applicationCategory: 'DeveloperApplication',
    operatingSystem: 'macOS, Linux',
    offers: { price: '0' },
    license: licenseUrl,
    sameAs: [repoUrl],
  });
});

test('only the home page carries structured data', async () => {
  for (const { file } of await pages()) {
    if (file === 'index.html') continue;
    expect(read(file), file).not.toContain('application/ld+json');
  }
});
