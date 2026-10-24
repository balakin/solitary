import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  productionHost,
  trackerAttributes,
  trackerSrc,
  trackRepositoryLink,
} from '../app/lib/analytics.ts';
import { repoUrl } from '../app/lib/shared.ts';

const id = '00000000-0000-4000-8000-000000000000';

test('a production build with a website ID loads the tracker for that ID', () => {
  const attributes = trackerAttributes(id, true);
  assert.ok(attributes);
  assert.equal(attributes.src, trackerSrc);
  assert.equal(attributes['data-website-id'], id);
});

test('the tracker reports only from the production host', () => {
  assert.equal(trackerAttributes(id, true)?.['data-domains'], productionHost);
});

test('the tracker drops query strings and fragments and honours Do Not Track', () => {
  const attributes = trackerAttributes(id, true);
  assert.equal(attributes?.['data-exclude-search'], 'true');
  assert.equal(attributes?.['data-exclude-hash'], 'true');
  assert.equal(attributes?.['data-do-not-track'], 'true');
});

test('development never loads the tracker', () => {
  assert.equal(trackerAttributes(id, false), null);
});

test('a build without a website ID loads no tracker', () => {
  assert.equal(trackerAttributes(undefined, true), null);
  assert.equal(trackerAttributes('', true), null);
  assert.equal(trackerAttributes('  ', true), null);
});

/** A click on a stand-in for an anchor, recording what reached `umami.track`. */
function click(attributes: Record<string, string>) {
  const sent: string[] = [];
  globalThis.window = { umami: { track: (event: string) => sent.push(event) } } as never;
  const link = {
    getAttribute: (name: string) => attributes[name] ?? null,
    hasAttribute: (name: string) => name in attributes,
  };
  trackRepositoryLink({ target: { closest: () => link } as never });
  return sent;
}

test('a link to the repository that the site does not render is counted', () => {
  assert.deepEqual(click({ href: repoUrl }), ['github_click']);
});

test('a link to the repository that carries its own event is left to the tracker', () => {
  assert.deepEqual(click({ href: repoUrl, 'data-umami-event': 'github_click' }), []);
});

test('links elsewhere, deeper into the repository included, are not counted', () => {
  assert.deepEqual(click({ href: `${repoUrl}/blob/main/LICENSE` }), []);
  assert.deepEqual(click({ href: '/docs' }), []);
});

test('a click outside any link is not counted', () => {
  globalThis.window = { umami: { track: () => assert.fail('tracked') } } as never;
  trackRepositoryLink({ target: { closest: () => null } as never });
  trackRepositoryLink({ target: null });
});
