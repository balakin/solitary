import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useEffect } from 'react';
import { describe, expect, test, vi } from 'vitest';
import {
  productionHost,
  trackerAttributes,
  trackerSrc,
  trackRepositoryLink,
} from '@/lib/analytics';
import { repoUrl } from '@/lib/shared';

const id = '00000000-0000-4000-8000-000000000000';

describe('trackerAttributes', () => {
  test('a production build with a website ID loads the tracker for that ID', () => {
    const attributes = trackerAttributes(id, true);
    expect(attributes?.src).toBe(trackerSrc);
    expect(attributes?.['data-website-id']).toBe(id);
  });

  test('the tracker reports only from the production host', () => {
    expect(trackerAttributes(id, true)?.['data-domains']).toBe(productionHost);
  });

  test('the tracker drops query strings and fragments and honours Do Not Track', () => {
    expect(trackerAttributes(id, true)).toMatchObject({
      'data-exclude-search': 'true',
      'data-exclude-hash': 'true',
      'data-do-not-track': 'true',
    });
  });

  test('development never loads the tracker', () => {
    expect(trackerAttributes(id, false)).toBeNull();
  });

  test('a build without a website ID loads no tracker', () => {
    expect(trackerAttributes(undefined, true)).toBeNull();
    expect(trackerAttributes('', true)).toBeNull();
    expect(trackerAttributes('  ', true)).toBeNull();
  });
});

/**
 * The listener as root.tsx installs it: on the document, for every click. The
 * wrapper cancels each click's navigation, which jsdom cannot perform and
 * reports; the click still bubbles to the document as it would on the site.
 */
function Listening({ children }: { children: React.ReactNode }) {
  useEffect(() => {
    document.addEventListener('click', trackRepositoryLink);
    return () => document.removeEventListener('click', trackRepositoryLink);
  }, []);
  // oxlint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-static-element-interactions
  return <div onClick={(event) => event.preventDefault()}>{children}</div>;
}

function renderWithTracker(ui: React.ReactNode) {
  const track = vi.fn();
  window.umami = { track };
  render(<Listening>{ui}</Listening>);
  return track;
}

describe('trackRepositoryLink', () => {
  test('a link to the repository that the site does not render is counted', async () => {
    const track = renderWithTracker(
      <a href={repoUrl}>
        <svg aria-hidden />
        GitHub
      </a>,
    );
    await userEvent.click(screen.getByRole('link', { name: 'GitHub' }));
    expect(track).toHaveBeenCalledExactlyOnceWith('github_click');
  });

  test('a click on an icon inside the link still counts it', async () => {
    const track = renderWithTracker(
      <a href={repoUrl} aria-label="GitHub">
        <svg data-testid="icon" />
      </a>,
    );
    await userEvent.click(screen.getByTestId('icon'));
    expect(track).toHaveBeenCalledExactlyOnceWith('github_click');
  });

  test('a link to the repository that carries its own event is left to the tracker', async () => {
    const track = renderWithTracker(
      <a href={repoUrl} data-umami-event="github_click">
        GitHub
      </a>,
    );
    await userEvent.click(screen.getByRole('link', { name: 'GitHub' }));
    expect(track).not.toHaveBeenCalled();
  });

  test('links elsewhere, deeper into the repository included, are not counted', async () => {
    const track = renderWithTracker(
      <>
        <a href={`${repoUrl}/blob/main/LICENSE`}>License</a>
        <a href="/docs">Docs</a>
      </>,
    );
    await userEvent.click(screen.getByRole('link', { name: 'License' }));
    await userEvent.click(screen.getByRole('link', { name: 'Docs' }));
    expect(track).not.toHaveBeenCalled();
  });

  test('a click outside any link is not counted', async () => {
    const track = renderWithTracker(<p>Just text</p>);
    await userEvent.click(screen.getByText('Just text'));
    expect(track).not.toHaveBeenCalled();
  });

  test('a blocked tracker turns a counted click into a no-op', async () => {
    render(
      <Listening>
        <a href={repoUrl}>GitHub</a>
      </Listening>,
    );
    await userEvent.click(screen.getByRole('link', { name: 'GitHub' }));
  });
});
