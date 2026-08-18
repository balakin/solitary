import { createFromSource } from 'fumadocs-core/search/server';
import { source } from '@/lib/source';

const server = createFromSource(source, {
  // https://docs.orama.com/docs/orama-js/supported-languages
  language: 'english',
});

// The whole index, written out at build time. There is no server behind this
// site, so the browser downloads it once and searches it locally rather than
// asking a running loader per keystroke.
export function loader() {
  return server.staticGET();
}
