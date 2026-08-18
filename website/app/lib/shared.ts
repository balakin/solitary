export const appName = 'Solitary';

// GitHub Pages serves this repository under /solitary rather than at the root of
// a domain. It is the react-router basename and Vite's base, so routes and
// bundled assets carry it on their own; anything under public/ is copied rather
// than resolved, so those URLs are prefixed by hand with `asset` below.
export const basename = '/solitary';

/** The public URL of a file served from `public/`. */
export function asset(path: string) {
  return `${basename}${path}`;
}
export const docsRoute = '/docs';
export const docsImageRoute = '/og/docs';
export const docsContentRoute = '/llms.mdx/docs';

export const siteTagline = 'Hypervisor-isolated cells for running coding agents off the leash';
export const siteDescription =
  'Run coding agents in disposable, hypervisor-isolated cells: no host mounts, secrets whitelisted per cell, default-deny egress, and an explicit way to move work in and out.';

// The home page's social image. A constant rather than a literal repeated in the
// route, the meta tag, and the prerender config: a path that is not prerendered
// 404s on a static host, and a social card that 404s fails silently.
export const homeImagePath = '/og/home/image.webp';

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
