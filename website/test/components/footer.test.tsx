import { describe, expect, test } from 'vitest';
import { Footer, PageCredits } from '@/components/footer';
import { author, licenseUrl, repoUrl } from '@/lib/shared';
import { findLink, renderPage } from './render';

// The home page and the 404 render Footer, the docs render PageCredits; both
// are the same credits, and both are checked so neither can drift.
describe.each([
  ['Footer', Footer],
  ['PageCredits', PageCredits],
])('%s', (_name, Credits) => {
  test('links to GitHub in a new tab, counted by the tracker', async () => {
    renderPage(Credits);
    const github = await findLink('GitHub', repoUrl);
    expect(github).toHaveAttribute('target', '_blank');
    expect(github).toHaveAttribute('rel', 'noreferrer');
    expect(github).toHaveAttribute('data-umami-event', 'github_click');
  });

  test('links to the licence in a new tab', async () => {
    renderPage(Credits);
    expect(await findLink('Apache 2.0 License', licenseUrl)).toHaveAttribute('target', '_blank');
  });

  test('links to the privacy policy within the site', async () => {
    renderPage(Credits);
    const privacy = await findLink('Privacy Policy', '/privacy');
    expect(privacy).not.toHaveAttribute('target');
    expect(privacy).not.toHaveAttribute('data-umami-event');
  });

  test('credits the author', async () => {
    renderPage(Credits);
    expect(await findLink(author.name, author.url)).toHaveAttribute('target', '_blank');
  });
});
