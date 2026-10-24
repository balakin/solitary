import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, test } from 'vitest';
import Home from '@/routes/home';
import { repoUrl } from '@/lib/shared';
import { findLink, renderPage } from './render';

test('the CTA to GitHub opens a new tab and is counted by the tracker', async () => {
  renderPage(Home);
  const github = await findLink('View on GitHub', repoUrl);
  expect(github).toHaveAttribute('target', '_blank');
  expect(github).toHaveAttribute('data-umami-event', 'github_click');
});

// Internal links are counted from onClick, not by attribute: the attribute makes
// the tracker reload the page in place of a client-side navigation.
test.each([
  ['Create your first cell', '/docs/quickstart'],
  ['other ways to install', '/docs/installation'],
])('"%s" carries no tracker attribute and counts an install click', async (name, href) => {
  const { track } = renderPage(Home);
  const link = await findLink(name, href);
  expect(link).not.toHaveAttribute('data-umami-event');

  await userEvent.click(link);
  expect(track).toHaveBeenCalledExactlyOnceWith('install_click');
});

/**
 * The copy button of the code block whose first token is `token`, once the
 * block is highlighted. Highlighting is asynchronous and replaces the block,
 * button included, when it is done: a button found earlier is detached by the
 * time it is clicked, and a click there reaches no handler at all.
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
  const { track } = renderPage(Home);
  await userEvent.setup().click(await copyButtonFor('curl'));
  expect(track).toHaveBeenCalledExactlyOnceWith('install_click');
});

test('copying the example cell definition counts nothing', async () => {
  const { track } = renderPage(Home);
  await userEvent.setup().click(await copyButtonFor('image'));
  expect(track).not.toHaveBeenCalled();
});
