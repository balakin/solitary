import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, test } from 'vitest';
import Home, { loader } from '@/routes/home';
import { repoUrl, videoUrl } from '@/lib/shared';
import { findLink, renderPage } from './render';

test('the CTA to GitHub opens a new tab and is counted by the tracker', async () => {
  renderPage(Home, loader);
  const github = await findLink('View on GitHub', repoUrl);
  expect(github).toHaveAttribute('target', '_blank');
  expect(github).toHaveAttribute('data-umami-event', 'github_click');
});

// A link, never an embedded player: YouTube's player would set cookies.
test('the video is a link to YouTube that opens a new tab and is counted', async () => {
  renderPage(Home, loader);
  const video = await findLink(/^Watch on YouTube/, videoUrl);
  expect(video).toHaveAttribute('target', '_blank');
  expect(video).toHaveAttribute('data-umami-event', 'video_click');
  expect(document.querySelector('iframe')).toBeNull();
});

// Internal links are counted from onClick, not by attribute: the attribute makes
// the tracker reload the page in place of a client-side navigation.
test.each([
  ['Create your first cell', '/docs/quickstart'],
  ['other ways to install', '/docs/installation'],
])('"%s" carries no tracker attribute and counts an install click', async (name, href) => {
  const { track } = renderPage(Home, loader);
  const link = await findLink(name, href);
  expect(link).not.toHaveAttribute('data-umami-event');

  await userEvent.click(link);
  expect(track).toHaveBeenCalledExactlyOnceWith('install_click');
});

/**
 * The copy button of the code block whose first token is `token`. The blocks
 * arrive highlighted from the loader, so the first token is its own span from
 * the first render; waiting covers the router rendering a tick later.
 */
async function copyButtonFor(token: string) {
  return waitFor(() => {
    const button = screen
      .getAllByRole('button', { name: /copy/i })
      .find((candidate) =>
        [...(candidate.closest('figure')?.querySelectorAll('pre span') ?? [])].some(
          (span) => span.children.length === 0 && span.textContent === token,
        ),
      );
    if (!button) throw new Error(`no highlighted block starting with "${token}"`);
    return button;
  });
}

test('copying the install command counts an install click', async () => {
  const { track } = renderPage(Home, loader);
  await userEvent.setup().click(await copyButtonFor('curl'));
  expect(track).toHaveBeenCalledExactlyOnceWith('install_click');
});

test('copying the example cell definition counts nothing', async () => {
  const { track } = renderPage(Home, loader);
  await userEvent.setup().click(await copyButtonFor('image'));
  expect(track).not.toHaveBeenCalled();
});
