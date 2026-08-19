import {
  isRouteErrorResponse,
  Links,
  Meta,
  Outlet,
  Scripts,
  ScrollRestoration,
} from 'react-router';
import { RootProvider } from 'fumadocs-ui/provider/react-router';
import type { Route } from './+types/root';
import './app.css';
import { isMarkdownPreferred, rewritePath } from 'fumadocs-core/negotiation';
import NotFound from './routes/not-found';
import { docsContentRoute, docsRoute } from '@/lib/shared';

// Every face is served from this origin: no third-party font request, so
// reading the docs does not announce itself to anyone but this host. Both are
// declared in app.css; these preloads start the two that are always needed
// before the stylesheet has been parsed.
export const links: Route.LinksFunction = () => [
  {
    rel: 'preload',
    as: 'font',
    type: 'font/woff2',
    href: '/fonts/InterVariable.woff2',
    crossOrigin: 'anonymous',
  },
  // The terminal art on the home page cannot draw its frame without this face.
  {
    rel: 'preload',
    as: 'font',
    type: 'font/woff2',
    href: '/fonts/JetBrainsMono-Regular.woff2',
    crossOrigin: 'anonymous',
  },
  // Declared rather than left to the browser's default guess: that guess is
  // /favicon.ico at the domain root, which is not where this site lives.
  { rel: 'icon', href: '/favicon.ico', type: 'image/x-icon' },
];

export function Layout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <Meta />
        <Links />
      </head>
      <body className="flex min-h-screen flex-col">
        {/* The footer is rendered by the pages that want one rather than here:
            the docs are a full-height layout, and a band below it is one more
            thing to scroll past on every page. */}
        <RootProvider
          search={{
            // There is no server to answer a query, so the dialog downloads the
            // index prerendered at this URL and searches it in the browser.
            options: { type: 'static', api: '/api/search' },
          }}
        >
          {children}
        </RootProvider>
        <ScrollRestoration />
        <Scripts />
      </body>
    </html>
  );
}

export default function App() {
  return <Outlet />;
}

export function ErrorBoundary({ error }: Route.ErrorBoundaryProps) {
  let message = 'Oops!';
  let details = 'An unexpected error occurred.';
  let stack: string | undefined;

  if (isRouteErrorResponse(error)) {
    if (error.status === 404) return <NotFound />;
    message = 'Error';
    details = error.statusText;
  } else if (import.meta.env.DEV && error && error instanceof Error) {
    details = error.message;
    stack = error.stack;
  }

  return (
    <main className="mx-auto w-full max-w-[1400px] p-4 pt-16">
      <h1>{message}</h1>
      <p>{details}</p>
      {stack && (
        <pre className="w-full overflow-x-auto p-4">
          <code>{stack}</code>
        </pre>
      )}
    </main>
  );
}

const { rewrite: rewriteDocs } = rewritePath(
  `${docsRoute}{/*path}`,
  `${docsContentRoute}{/*path}/content.md`,
);
const { rewrite: rewriteSuffix } = rewritePath(
  `${docsRoute}{/*path}.md`,
  `${docsContentRoute}{/*path}/content.md`,
);
const serverMiddleware: Route.MiddlewareFunction = async ({ request }, next) => {
  const url = new URL(request.url);
  const suffixPath = rewriteSuffix(url.pathname);
  if (suffixPath) return Response.redirect(new URL(suffixPath, url));

  if (isMarkdownPreferred(request)) {
    const docsPath = rewriteDocs(url.pathname);
    // this URL has two representations selected by `Accept`, and the headers of
    // `Response.redirect()` are immutable, so build the response directly
    if (docsPath)
      return new Response(null, {
        status: 302,
        headers: {
          Location: new URL(docsPath, url).toString(),
          Vary: 'Accept',
        },
      });
  }

  return next();
};
export const middleware = [serverMiddleware];
