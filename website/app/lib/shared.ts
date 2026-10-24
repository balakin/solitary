export const appName = 'Solitary';

export const docsRoute = '/docs';
// The docs' first page. Not /docs itself: every page is served as a file of
// its own, and /docs has to stay a directory to hold the others, so the page
// that would live there lives here and /docs redirects to it.
export const docsHome = `${docsRoute}/overview`;
export const docsImageRoute = '/og/docs';
export const docsContentRoute = '/llms.mdx/docs';

export const siteTagline = 'Run coding agents on a VM, not on your machine';
export const siteDescription =
  'Run coding agents in disposable, hypervisor-isolated cells: no host mounts, secrets whitelisted per cell, default-deny egress, and an explicit way to move work in and out.';

// Absolute because the social images are read by crawlers, not browsers: the
// Open Graph protocol asks for a full URL, and Facebook and LinkedIn drop a
// relative one without saying so.
export const siteUrl = 'https://solitary.balakin.io';

/**
 * The one address a page is indexed under, for its canonical link, its og:url
 * and its sitemap entry alike: the bare path, the one fumadocs builds every
 * link with and the address bar shows. Pages serves it without a redirect
 * because scripts/pages-artifact.mjs turns each page into a file of its own.
 */
export function canonicalUrl(path: string) {
  return `${siteUrl}${path === '/' ? '/' : path.replace(/\/$/, '')}`;
}

export function canonicalMeta(path: string) {
  const url = canonicalUrl(path);
  return [
    { tagName: 'link', rel: 'canonical', href: url },
    { property: 'og:url', content: url },
  ];
}

// The site's cover: a file in public/ rather than a generated route, drawn at
// twice the 1200×630 card size so it stays sharp on a high-density display.
export const siteImage = {
  url: `${siteUrl}/og.png`,
  width: 2000,
  height: 1050,
  alt: 'Solitary: Let coding agents run free. Keep your machine out of reach.',
};

export function siteImageMeta() {
  return [
    { property: 'og:image', content: siteImage.url },
    { property: 'og:image:width', content: String(siteImage.width) },
    { property: 'og:image:height', content: String(siteImage.height) },
    { property: 'og:image:alt', content: siteImage.alt },
    { name: 'twitter:card', content: 'summary_large_image' },
    { name: 'twitter:image', content: siteImage.url },
    { name: 'twitter:image:alt', content: siteImage.alt },
  ];
}

// fill this with your actual GitHub info, for example:
export const gitConfig = {
  user: 'balakin',
  repo: 'solitary',
  branch: 'main',
};

export const author = {
  name: 'Dmitrii Balakin',
  url: `https://github.com/${gitConfig.user}`,
};

export const repoUrl = `https://github.com/${gitConfig.user}/${gitConfig.repo}`;
export const licenseUrl = `${repoUrl}/blob/${gitConfig.branch}/LICENSE`;

export function getPageImagePath(slugs: string[], locale?: string) {
  return (
    '/' + [locale, ...docsImageRoute.split('/'), ...slugs, 'image.webp'].filter(Boolean).join('/')
  );
}

// The plain-Markdown representation of a page, for agents and for the copy
// button. Built here rather than at each call site so the prerender config and
// the loader cannot drift apart: a path that is not prerendered 404s on a
// static host, and nothing would say so.
export function getPageContentPath(slugs: string[], locale?: string) {
  return (
    '/' + [locale, ...docsContentRoute.split('/'), ...slugs, 'content.md'].filter(Boolean).join('/')
  );
}
