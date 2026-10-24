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
  expect(existsSync(join(out, 'privacy', 'index.html'))).toBe(true);
});

test('the privacy policy has its own title and description', () => {
  const html = read('privacy/index.html');
  expect(html).toContain('<title>Privacy Policy | Solitary</title>');
  expect(html).toMatch(/<meta name="description" content="[^"]+"/);
});

// The footer's links are tested on the component; this is that every page
// that should render it does.
test.each(['index.html', 'privacy/index.html', 'docs/index.html', '404.html'])(
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
  ['privacy/index.html', `${site}/og.png`],
  ['docs/index.html', `${site}/og/docs/image.webp`],
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
  const shipped: string[] = [];

  for await (const page of glob('**/index.html', { cwd: out })) {
    shipped.push(`${site}/${page.slice(0, -'index.html'.length)}`);
  }

  expect(new Set(listed)).toEqual(new Set(shipped));
  expect(listed, 'the sitemap lists a page twice').toHaveLength(shipped.length);
});

test('robots.txt allows crawling and points at the sitemap', () => {
  const robots = read('robots.txt');
  expect(robots).not.toMatch(/^Disallow: \/\S/m);
  expect(robots).toMatch(/^Sitemap: https:\/\/solitary\.balakin\.io\/sitemap\.xml$/m);
});
