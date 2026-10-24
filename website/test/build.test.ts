// Checks the prerendered site in build/client — what GitHub Pages serves — rather
// than the components, so it fails on what a visitor would actually receive.
// Run `pnpm build:pages` first; CI runs it in that order.

import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { glob } from 'node:fs/promises';
import { join } from 'node:path';
import { before, test } from 'node:test';

const out = 'build/client';
const repoUrl = 'https://github.com/balakin/solitary';
const licenseUrl = `${repoUrl}/blob/main/LICENSE`;

const read = (path: string) => readFileSync(join(out, path), 'utf8');

/** Every opening tag in `html` whose name is `tag` and which contains `needle`. */
function tags(html: string, tag: string, needle: string) {
  return [...html.matchAll(new RegExp(`<${tag}\\b[^>]*>`, 'g'))]
    .map(([match]) => match)
    .filter((match) => match.includes(needle));
}

/** The opening tag of the link to `href` whose text is `text`. */
function link(html: string, href: string, text: string) {
  const match = html.match(new RegExp(`(<a\\b[^>]*href="${href}"[^>]*>)(?:<!-- -->)?${text}<`));
  assert.ok(match, `no link to ${href} reading "${text}"`);
  return match[1];
}

before(() => {
  assert.ok(
    existsSync(join(out, '404.html')),
    `${out} is missing or incomplete: run \`pnpm build:pages\` first`,
  );
});

test('/privacy is prerendered, so it loads directly and on refresh', () => {
  assert.ok(existsSync(join(out, 'privacy', 'index.html')));
});

test('the privacy policy names its controller and contact', () => {
  const html = read('privacy/index.html');
  assert.match(html, /<title>Privacy Policy — Solitary<\/title>/);
  assert.match(html, /<meta name="description" content="[^"]+"/);
  assert.match(html, /<h1>Privacy Policy<\/h1>/);
  assert.match(html, /Dmitrii Balakin/);
  assert.match(html, /Serbia/);
  assert.match(html, /href="mailto:privacy@balakin.io"/);
});

test('the privacy policy carries only a last-updated date', () => {
  const html = read('privacy/index.html');
  assert.match(
    html,
    /Last updated: (?:<!-- -->)?<time dateTime="2026-10-24">October 24, 2026<\/time>/,
  );
  assert.doesNotMatch(html, /effective/i);
});

test('every footer links to GitHub, the licence and the privacy policy', () => {
  for (const page of ['index.html', 'privacy/index.html', 'docs/index.html', '404.html']) {
    const html = read(page);
    const github = link(html, repoUrl, 'GitHub');
    assert.match(github, /target="_blank"/, page);
    assert.match(github, /data-umami-event="github_click"/, page);
    assert.match(link(html, licenseUrl, 'Apache 2.0 License'), /target="_blank"/, page);
    link(html, '/privacy', 'Privacy Policy');
  }
});

test('the home page CTA to GitHub is tracked and still opens a new tab', () => {
  const github = link(read('index.html'), repoUrl, 'View on GitHub');
  assert.match(github, /target="_blank"/);
  assert.match(github, /data-umami-event="github_click"/);
});

// Internal links are tracked from onClick, not by attribute: the attribute makes
// the tracker reload the page in place of a client-side navigation.
test('internal CTAs carry no tracker attribute', () => {
  const html = read('index.html');
  assert.doesNotMatch(link(html, '/docs/quickstart', 'Create your first cell'), /data-umami/);
  assert.doesNotMatch(link(html, '/docs/installation', 'other ways to install'), /data-umami/);
});

test('each page loads the tracker once, and only when the build had a website ID', async () => {
  const id = process.env.VITE_UMAMI_WEBSITE_ID?.trim();
  let pages = 0;

  for await (const page of glob('**/*.html', { cwd: out })) {
    const scripts = tags(read(page), 'script', 'cloud.umami.is');
    pages++;

    if (!id) {
      assert.equal(scripts.length, 0, `${page} loads a tracker with no website ID`);
      continue;
    }
    assert.equal(scripts.length, 1, `${page} should load the tracker exactly once`);
    assert.match(scripts[0], /src="https:\/\/cloud.umami.is\/script.js"/);
    assert.match(scripts[0], new RegExp(`data-website-id="${id}"`));
    assert.match(scripts[0], /data-domains="solitary.balakin.io"/);
    assert.match(scripts[0], /defer/);
  }

  assert.ok(pages > 1, 'no prerendered pages found');
});

// Crawlers fetch the image by the URL in the tag, so it has to be absolute and
// has to name a file the build actually ships: either mistake leaves a link
// preview with no picture and no error anywhere.
test('every page names an absolute social image that the build ships', () => {
  const site = 'https://solitary.balakin.io';
  const pages = {
    'index.html': `${site}/og.png`,
    'privacy/index.html': `${site}/og.png`,
    'docs/index.html': `${site}/og/docs/image.webp`,
  };

  for (const [page, image] of Object.entries(pages)) {
    const html = read(page);
    assert.equal(tags(html, 'meta', `property="og:image" content="${image}"`).length, 1, page);
    assert.equal(tags(html, 'meta', `name="twitter:image" content="${image}"`).length, 1, page);
    assert.ok(existsSync(join(out, image.slice(site.length))), `${image} is not in the build`);
  }
});
