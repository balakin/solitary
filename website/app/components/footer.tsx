import { author, licenseUrl, repoUrl } from '@/lib/shared';

const linkStyle = 'text-fd-foreground underline-offset-4 hover:underline';

/**
 * Site footer. Rendered once in the root layout, so it sits below both the home
 * page and the docs — the body is a min-height flex column, which keeps it at
 * the bottom on short pages rather than floating mid-screen.
 */
export function Footer() {
  return (
    <footer className="mt-auto border-t border-fd-border bg-fd-card/40">
      <div className="mx-auto flex max-w-6xl flex-col gap-3 px-6 py-8 text-sm text-fd-muted-foreground sm:flex-row sm:items-center sm:justify-between">
        <p>
          Made by{' '}
          <a className={linkStyle} href={author.url} target="_blank" rel="noreferrer">
            {author.name}
          </a>
        </p>
        <nav className="flex gap-5">
          <a className={linkStyle} href={repoUrl} target="_blank" rel="noreferrer">
            GitHub
          </a>
          <a className={linkStyle} href={licenseUrl} target="_blank" rel="noreferrer">
            Apache 2.0
          </a>
        </nav>
      </div>
    </footer>
  );
}
