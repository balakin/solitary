import assert from 'node:assert/strict';
import { test } from 'node:test';
import { productionHost, trackerAttributes, trackerSrc } from '../app/lib/analytics.ts';

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
