import { author, licenseUrl, repoUrl } from '@/lib/shared';

const linkStyle = 'text-fd-foreground underline-offset-4 hover:underline';

/** Who made this and under what licence, laid out as one row on wide screens. */
function Credits() {
  return (
    <>
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
    </>
  );
}

/**
 * Site footer, for the pages that are one column of content: the home page and
 * the 404. Both layouts are a flex column that fills the viewport, so `mt-auto`
 * keeps this at the bottom on a short page rather than floating mid-screen.
 *
 * Deliberately not rendered in the root layout. The docs are a full-height
 * layout with their own sidebar and table of contents, and a band underneath it
 * is a second thing to scroll past on every page; there the footer belongs to
 * the article instead — see `PageFooter`.
 */
export function Footer() {
  return (
    <footer className="mt-auto border-t border-fd-border bg-fd-card/40">
      <div className="mx-auto flex max-w-6xl flex-col gap-3 px-6 py-8 text-sm text-fd-muted-foreground sm:flex-row sm:items-center sm:justify-between">
        <Credits />
      </div>
    </footer>
  );
}

/**
 * The same credits at the end of a docs page, inside the article column rather
 * than below the layout: it ends where the text ends, aligned with it, and adds
 * nothing to scroll past for a reader who stops reading earlier.
 *
 * Rendered under the links to the previous and next pages, which are what
 * someone who reached the end of a page is looking for. See `routes/docs.tsx`,
 * which slots it into fumadocs' own page footer.
 */
export function PageCredits() {
  return (
    <footer className="mt-2 flex flex-col gap-3 text-sm text-fd-muted-foreground sm:flex-row sm:items-center sm:justify-between">
      <Credits />
    </footer>
  );
}
