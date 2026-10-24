import type { Route } from './+types/not-found';
import { HomeLayout } from 'fumadocs-ui/layouts/home';
import { baseOptions } from '@/lib/layout.shared';
import { DefaultNotFound } from 'fumadocs-ui/layouts/home/not-found';
import { Footer } from '@/components/footer';

export function meta(_args: Route.MetaArgs) {
  return [{ title: 'Not Found' }];
}

// Every page used to be served at its slashed address, and those addresses
// were indexed and shared before each page became a file of its own. Pages has
// nothing at them now and answers with this file, so the browser is sent on to
// the bare address before the app starts. Inline and in the HTML, because it
// has to run as the document is parsed: a script React adds on the client never
// runs at all. A slashed path that is still missing without its slash lands
// here again and stays, so it cannot loop.
const dropTrailingSlash = `if (location.pathname.length > 1 && location.pathname.endsWith('/')) location.replace(location.pathname.replace(/\\/+$/, '') + location.search + location.hash);`;

export default function NotFound() {
  return (
    <HomeLayout {...baseOptions()}>
      {/* oxlint-disable-next-line react/no-danger */}
      <script dangerouslySetInnerHTML={{ __html: dropTrailingSlash }} />
      <DefaultNotFound />
      <Footer />
    </HomeLayout>
  );
}
