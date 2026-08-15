import type { BaseLayoutProps } from 'fumadocs-ui/layouts/shared';
import { appName, gitConfig } from './shared';

export function baseOptions(): BaseLayoutProps {
  return {
    nav: {
      // JSX supported
      title: appName,
    },
    githubUrl: `https://github.com/${gitConfig.user}/${gitConfig.repo}`,
  };
}

/**
 * The home page's header links.
 *
 * Deliberately not in `baseOptions`: the docs sidebar renders the same link
 * items as the page tree above it, so shared links show up there a second time
 * — "Documentation" over "Overview", "Quickstart" over the page itself. The
 * docs already have a sidebar; only the home page needs a way in from the
 * header.
 */
export function homeOptions(): BaseLayoutProps {
  return {
    ...baseOptions(),
    links: [
      { text: 'Documentation', url: '/docs' },
      { text: 'Quickstart', url: '/docs/quickstart' },
    ],
  };
}
