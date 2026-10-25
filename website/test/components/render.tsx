import { render, screen } from '@testing-library/react';
import { RootProvider } from 'fumadocs-ui/provider/react-router';
import { createRoutesStub } from 'react-router';
import { vi } from 'vitest';

/**
 * Renders a page at `/` the way root.tsx does: inside a router, which every
 * `Link` needs, and fumadocs' provider, which its layouts read the router
 * through. A stand-in for the tracker lets a test see what a click sent. The
 * router renders a tick later, so a test waits with a `findBy` query first.
 */
export function renderPage(Page: React.ComponentType) {
  const track = vi.fn();
  window.umami = { track };

  const Stub = createRoutesStub([
    {
      path: '/',
      Component: () => (
        <RootProvider>
          <Page />
        </RootProvider>
      ),
    },
  ]);
  render(<Stub initialEntries={['/']} />);
  return { track };
}

/** The link to `href` whose accessible name is `name`. */
export async function findLink(name: string | RegExp, href: string) {
  const links = await screen.findAllByRole('link', { name });
  const link = links.find((candidate) => candidate.getAttribute('href') === href);
  if (!link) throw new Error(`no link to ${href} reading "${name}"`);
  return link;
}
